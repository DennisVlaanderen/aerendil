// Contributor License Agreement check for pull requests, run by
// .github/workflows/cla.yml through actions/github-script.
//
// Who has to sign: the pull request's author plus every GitHub account that
// authored one of its commits. Exempt: the repository owner (the licensor),
// bot accounts, and the AI coding agents listed in AGENT_LOGINS. An agent's
// commits are covered by the signature of the person who opened the pull
// request, since they're the one submitting that work. Commits whose author
// email isn't linked to any GitHub account are covered the same way.
//
// Signatures live in SIGNATURES_PATH on the SIGNATURES_BRANCH branch, which
// is created (as an orphan branch, with no history shared with main) the
// first time anyone signs. Keeping them off main means the check never has
// to get past main's branch protection. Every signature is a commit there,
// so the branch history doubles as an audit log.
//
// The script never checks out or runs pull request code; it only reads and
// writes through the GitHub API.

const CLA_VERSION = '1';
const SIGNATURES_BRANCH = 'cla-signatures';
const SIGNATURES_PATH = 'signatures.json';
const SIGN_PHRASE = 'I have read the CLA and I agree to its terms.';
const RECHECK_PHRASE = 'recheck';
const STATUS_CONTEXT = 'CLA';
const COMMENT_MARKER = '<!-- aerendil-cla-check -->';
const AGENT_LOGINS = new Set(['claude']);

// Case, surrounding whitespace, and a trailing period don't matter when
// matching a comment against SIGN_PHRASE or RECHECK_PHRASE.
const normalize = (text) =>
  text.trim().replace(/\s+/g, ' ').replace(/\.$/, '').toLowerCase();

module.exports = async ({ github, context, core }) => {
  const { owner, repo } = context.repo;
  const ownerId = context.payload.repository.owner.id;
  const claUrl = `${context.serverUrl}/${owner}/${repo}/blob/${context.payload.repository.default_branch}/CLA.md`;

  const isExempt = (user) =>
    user.id === ownerId || user.type === 'Bot' || AGENT_LOGINS.has(user.login);

  async function loadSignatures() {
    try {
      const { data } = await github.rest.repos.getContent({
        owner, repo, path: SIGNATURES_PATH, ref: SIGNATURES_BRANCH,
      });
      const doc = JSON.parse(Buffer.from(data.content, 'base64').toString('utf8'));
      return { sha: data.sha, doc };
    } catch (err) {
      if (err.status === 404) return { sha: undefined, doc: { signatures: [] } };
      throw err;
    }
  }

  async function branchExists() {
    try {
      await github.rest.repos.getBranch({ owner, repo, branch: SIGNATURES_BRANCH });
      return true;
    } catch (err) {
      if (err.status === 404) return false;
      throw err;
    }
  }

  async function createSignaturesBranch(content, message) {
    const { data: blob } = await github.rest.git.createBlob({
      owner, repo, content, encoding: 'utf-8',
    });
    const { data: tree } = await github.rest.git.createTree({
      owner, repo,
      tree: [{ path: SIGNATURES_PATH, mode: '100644', type: 'blob', sha: blob.sha }],
    });
    const { data: commit } = await github.rest.git.createCommit({
      owner, repo, message, tree: tree.sha, parents: [],
    });
    await github.rest.git.createRef({
      owner, repo, ref: `refs/heads/${SIGNATURES_BRANCH}`, sha: commit.sha,
    });
  }

  // Retries on a conflict, which means another run wrote a signature (or
  // created the branch) between our read and our write.
  async function saveSignature(entry) {
    for (let attempt = 1; ; attempt++) {
      const { sha, doc } = await loadSignatures();
      if (doc.signatures.some((s) => s.id === entry.id && s.claVersion === entry.claVersion)) {
        return;
      }
      doc.signatures.push(entry);
      const content = `${JSON.stringify(doc, null, 2)}\n`;
      const message = `${entry.login} signed CLA version ${entry.claVersion} in pull request ${entry.pullRequest}`;
      try {
        if (await branchExists()) {
          await github.rest.repos.createOrUpdateFileContents({
            owner, repo, path: SIGNATURES_PATH, branch: SIGNATURES_BRANCH,
            message, sha, content: Buffer.from(content).toString('base64'),
          });
        } else {
          await createSignaturesBranch(content, message);
        }
        return;
      } catch (err) {
        if (attempt < 3 && (err.status === 409 || err.status === 422)) continue;
        throw err;
      }
    }
  }

  let pr;
  if (context.eventName === 'issue_comment') {
    const { issue, comment } = context.payload;
    if (!issue.pull_request) return;
    const body = normalize(comment.body);
    const signing = body === normalize(SIGN_PHRASE);
    if (!signing && body !== RECHECK_PHRASE) return;

    ({ data: pr } = await github.rest.pulls.get({ owner, repo, pull_number: issue.number }));
    if (signing && !isExempt(comment.user)) {
      await saveSignature({
        login: comment.user.login,
        id: comment.user.id,
        claVersion: CLA_VERSION,
        pullRequest: pr.number,
        commentUrl: comment.html_url,
        signedAt: comment.created_at,
      });
      core.info(`Recorded a CLA signature for ${comment.user.login}.`);
    }
  } else {
    pr = context.payload.pull_request;
  }
  if (pr.state !== 'open') return;

  const required = new Map();
  const addSigner = (user) => {
    if (user && !isExempt(user)) required.set(user.id, user.login);
  };
  addSigner(pr.user);
  const commits = await github.paginate(github.rest.pulls.listCommits, {
    owner, repo, pull_number: pr.number, per_page: 100,
  });
  for (const commit of commits) addSigner(commit.author);

  const { doc } = await loadSignatures();
  const signed = new Set(
    doc.signatures.filter((s) => s.claVersion === CLA_VERSION).map((s) => s.id),
  );
  const missing = [...required].filter(([id]) => !signed.has(id)).map(([, login]) => login);

  const comments = await github.paginate(github.rest.issues.listComments, {
    owner, repo, issue_number: pr.number, per_page: 100,
  });
  const existing = comments.find(
    (c) => c.user.type === 'Bot' && c.body.includes(COMMENT_MARKER),
  );

  let body;
  if (missing.length > 0) {
    body = [
      COMMENT_MARKER,
      '### Contributor License Agreement',
      '',
      `Thanks for your contribution! Before it can be merged, everyone listed below needs to sign the [Aerendil Contributor License Agreement](${claUrl}). You only need to sign once; it covers all your future contributions to Aerendil.`,
      '',
      ...missing.map((login) => `- @${login}`),
      '',
      'To sign, read the agreement, then post a comment on this pull request containing exactly:',
      '',
      '```text',
      SIGN_PHRASE,
      '```',
      '',
      `Comment \`${RECHECK_PHRASE}\` to run the check again.`,
    ].join('\n');
  } else if (existing) {
    body = [
      COMMENT_MARKER,
      '### Contributor License Agreement',
      '',
      'Everyone on this pull request has signed the CLA. Thank you!',
    ].join('\n');
  }

  if (body && existing) {
    if (existing.body !== body) {
      await github.rest.issues.updateComment({ owner, repo, comment_id: existing.id, body });
    }
  } else if (body) {
    await github.rest.issues.createComment({ owner, repo, issue_number: pr.number, body });
  }

  await github.rest.repos.createCommitStatus({
    owner, repo, sha: pr.head.sha,
    context: STATUS_CONTEXT,
    state: missing.length > 0 ? 'failure' : 'success',
    description: missing.length > 0
      ? `Waiting on a CLA signature from ${missing.join(', ')}`.slice(0, 140)
      : 'All contributors have signed the CLA',
    target_url: claUrl,
  });
};
