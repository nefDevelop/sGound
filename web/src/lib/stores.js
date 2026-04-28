import { writable, derived } from 'svelte/store';

export const toast = writable(null);

const isMobile = typeof navigator !== 'undefined' && /iPhone|iPad|iPod|Android/i.test(navigator.userAgent);
const defaultQuality = isMobile ? '128k' : 'original';

export const audioQuality = writable(localStorage.getItem('audioQuality') || defaultQuality);

audioQuality.subscribe(value => {
    localStorage.setItem('audioQuality', value);
});

export const effectiveQuality = derived(audioQuality, ($quality) => {
    if ($quality !== 'auto') return $quality;
    
    // Auto-detection based on network speed (where available)
    const nav = typeof navigator !== 'undefined' ? navigator : null;
    if (nav && 'connection' in nav) {
        // @ts-ignore
        const type = nav.connection.effectiveType; // '4g', '3g', '2g', 'slow-2g'
        if (type === '4g') return 'original';
        if (type === '3g') return '192k';
        return '128k'; // Fallback for 2g or slow connections
    }
    
    // Basic fallback for browsers without Network Information API (like Safari)
    return isMobile ? '128k' : 'original';
});

// Playback state
export const queue = writable([]);
export const currentIndex = writable(-1);
export const isPlaying = writable(false);
export const repeatMode = writable('none'); // 'none', 'all', 'one'
export const isShuffle = writable(false);
export const inboxCount = writable(0);
export const rawCount = writable(0);
export const coverRefreshTrigger = writable(Date.now());
export const volume = writable(parseFloat(localStorage.getItem('volume')) || 1.0);

// Navigation & Auth
export const currentView = writable("dashboard");
export const viewParams = writable({});
export const isLoggedIn = writable(!!localStorage.getItem('username'));
export const setupRequired = writable(false);

export async function apiFetch(url, options = {}) {
    const res = await fetch(url, options);
    if (res.status === 401) {
        isLoggedIn.set(false);
        localStorage.removeItem('username');
        return res;
    }
    return res;
}

volume.subscribe(v => {
    localStorage.setItem('volume', v.toString());
});

export function showToast(message, type = 'info') {
    toast.set({ message, type });
    setTimeout(() => {
        toast.set(null);
    }, 4000);
}
