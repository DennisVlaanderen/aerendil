// Keys must stay '1'-'6': the backend only accepts those palette numbers.
export const ENVIRONMENT_COLORS: Record<string, string> = {
	'1': '#ef4444', // red
	'2': '#fa8000', // amber
	'3': '#ffd800', // yellow
	'4': '#22c55e', // green
	'5': '#3b82f6', // blue
	'6': '#8b5cf6' // violet
};

const hexPattern = /^#([0-9a-f]{3}){1,2}$/i;

export const resolveEnvironmentColor = (color?: string): string | null => {
	if (!color) return null;
	if (color.startsWith('#')) return hexPattern.test(color) ? color : null;
	return ENVIRONMENT_COLORS[color] ?? null;
};
