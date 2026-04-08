import { writable } from 'svelte/store';

export const toast = writable(null);

const isMobile = typeof navigator !== 'undefined' && /iPhone|iPad|iPod|Android/i.test(navigator.userAgent);
const defaultQuality = isMobile ? '128k' : 'original';

export const audioQuality = writable(localStorage.getItem('audioQuality') || defaultQuality);

audioQuality.subscribe(value => {
    localStorage.setItem('audioQuality', value);
});

// Playback state
export const queue = writable([]);
export const currentIndex = writable(-1);
export const isPlaying = writable(false);
export const repeatMode = writable('none'); // 'none', 'all', 'one'
export const isShuffle = writable(false);
export const inboxCount = writable(0);

export function showToast(message, type = 'info') {
    toast.set({ message, type });
    setTimeout(() => {
        toast.set(null);
    }, 4000);
}
