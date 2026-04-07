import { writable } from 'svelte/store';

export const toast = writable(null);

const isMobile = typeof navigator !== 'undefined' && /iPhone|iPad|iPod|Android/i.test(navigator.userAgent);
const defaultQuality = isMobile ? '128k' : 'original';

export const audioQuality = writable(localStorage.getItem('audioQuality') || defaultQuality);

audioQuality.subscribe(value => {
    localStorage.setItem('audioQuality', value);
});

export function showToast(message, type = 'info') {
    toast.set({ message, type });
    setTimeout(() => {
        toast.set(null);
    }, 4000);
}
