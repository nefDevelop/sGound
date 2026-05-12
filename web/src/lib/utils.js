/**
 * Copies text to the clipboard with fallback for non-secure contexts.
 * @param {string} text - The text to copy.
 * @returns {Promise<boolean>} - Whether the copy was successful.
 */
export async function copyToClipboard(text) {
    if (!text) return false;

    // Try modern API first (requires secure context)
    if (navigator.clipboard && navigator.clipboard.writeText) {
        try {
            await navigator.clipboard.writeText(text);
            return true;
        } catch (err) {
            console.error('Failed to copy using navigator.clipboard:', err);
        }
    }

    // Fallback for non-secure contexts or older browsers
    try {
        const textArea = document.createElement("textarea");
        textArea.value = text;
        
        // Ensure textarea is not visible but part of the DOM
        textArea.style.position = "fixed";
        textArea.style.left = "-999999px";
        textArea.style.top = "-999999px";
        document.body.appendChild(textArea);
        
        textArea.focus();
        textArea.select();
        
        const successful = document.execCommand('copy');
        document.body.removeChild(textArea);
        
        if (successful) return true;
    } catch (err) {
        console.error('Fallback copy failed:', err);
    }

    return false;
}
