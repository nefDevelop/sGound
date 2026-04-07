/**
 * Material Design 3 Expressive Palette logic
 */

export function hexToHsl(hex) {
    let r = parseInt(hex.slice(1, 3), 16) / 255;
    let g = parseInt(hex.slice(3, 5), 16) / 255;
    let b = parseInt(hex.slice(5, 7), 16) / 255;

    let max = Math.max(r, g, b), min = Math.min(r, g, b);
    let h, s, l = (max + min) / 2;

    if (max === min) {
        h = s = 0;
    } else {
        let d = max - min;
        s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
        switch (max) {
            case r: h = (g - b) / d + (g < b ? 6 : 0); break;
            case g: h = (b - r) / d + 2; break;
            case b: h = (r - g) / d + 4; break;
        }
        h /= 6;
    }
    return [h * 360, s * 100, l * 100];
}

export function hslToHex(h, s, l) {
    l /= 100;
    const a = s * Math.min(l, 1 - l) / 100;
    const f = n => {
        const k = (n + h / 30) % 12;
        const color = l - a * Math.max(Math.min(k - 3, 9 - k, 1), -1);
        return Math.round(255 * color).toString(16).padStart(2, '0');
    };
    return `#${f(0)}${f(8)}${f(4)}`;
}

export function generatePalette(seedHex) {
    const [h, s, l] = hexToHsl(seedHex);

    // Expressive logic:
    // Secondary: shift hue slightly, reduce saturation
    const secondaryHex = hslToHex((h + 20) % 360, Math.max(s - 20, 20), Math.max(l - 5, 40));
    
    // Tertiary: shift hue significantly (vibrant contrast)
    const tertiaryHex = hslToHex((h + 120) % 360, Math.min(s + 10, 90), Math.max(l + 5, 50));

    return {
        primary: seedHex,
        secondary: secondaryHex,
        tertiary: tertiaryHex,
        primary10: `rgba(${parseInt(seedHex.slice(1, 3), 16)}, ${parseInt(seedHex.slice(3, 5), 16)}, ${parseInt(seedHex.slice(5, 7), 16)}, 0.1)`
    };
}

export function applyTheme(palette) {
    const root = document.documentElement;
    root.style.setProperty('--primary', palette.primary);
    root.style.setProperty('--primary-10', palette.primary10);
    root.style.setProperty('--secondary', palette.secondary);
    root.style.setProperty('--tertiary', palette.tertiary);
}
