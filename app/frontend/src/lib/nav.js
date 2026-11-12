import { writable } from 'svelte/store';

// Current top-level tab: 'dashboard' | 'health' | 'runs' | 'compare'.
export const page = writable('dashboard');

// Run IDs picked in the Runs tab to open in Compare.
export const compareSelection = writable([]);
