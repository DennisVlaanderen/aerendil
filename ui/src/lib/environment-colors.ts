export const ENVIRONMENT_COLORS: Record<string, string> = {
    '1': '#FF0000', // red
    '2': '#CA8000', // amber
    '3': '#FFFF00', // yellow
    '4': '#00FF00', // green
    '5': '#0000FF', // blue
    '6': '#800080'  // violet
}

export const resolveEnvironmentColor = (color: string): string | null => {
    if (color.startsWith('#')) {
        // Validate hex color
        const hexRegex = /^#([0-9A-Fa-f]{6}|[0-9A-Fa-f]{3})$/;
        return hexRegex.test(color) ? color : null;
    }

    // Check if it's a palette number
    if (ENVIRONMENT_COLORS[color]) {
        return ENVIRONMENT_COLORS[color];
    }

    return null;
}