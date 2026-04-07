/**
 * Material Design 3 Tonal Harmony logic
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

    // MD3 Tonal Harmony Logic
    const primary = seedHex;
    const secondary = hslToHex(h, Math.min(s * 0.4, 25), 60);
    const tertiary = hslToHex((h + 60) % 360, Math.min(s + 10, 50), 65);
    
    // GNOME / MD3 Surfaces (Dark Mode by default)
    const surface = hslToHex(h, 6, 8); // Fondo base ultra oscuro con tinte
    const surfaceContainer = hslToHex(h, 8, 12); // Contenedores (estilo Adwaita)
    const surfaceContainerHigh = hslToHex(h, 10, 18); // Tarjetas
    const surfaceContainerHighest = hslToHex(h, 12, 24); // Tooltips, Popovers, Swatches

    return {
        primary,
        secondary,
        tertiary,
        surface,
        surfaceContainer,
        surfaceContainerHigh,
        surfaceContainerHighest,
        primary10: `rgba(${parseInt(seedHex.slice(1, 3), 16)}, ${parseInt(seedHex.slice(3, 5), 16)}, ${parseInt(seedHex.slice(5, 7), 16)}, 0.1)`,
        onSurface: '#ffffff',
        onSurfaceVariant: 'rgba(255,255,255,0.7)'
    };
}

export function applyTheme(palette) {
    const root = document.documentElement;
    root.style.setProperty('--primary', palette.primary);
    root.style.setProperty('--primary-10', palette.primary10);
    root.style.setProperty('--secondary', palette.secondary);
    root.style.setProperty('--tertiary', palette.tertiary);
    root.style.setProperty('--surface', palette.surface);
    root.style.setProperty('--surface-container', palette.surfaceContainer);
    root.style.setProperty('--surface-container-high', palette.surfaceContainerHigh);
    root.style.setProperty('--surface-container-highest', palette.surfaceContainerHighest);
    root.style.setProperty('--text-main', palette.onSurface);
    root.style.setProperty('--text-muted', palette.onSurfaceVariant);
}
