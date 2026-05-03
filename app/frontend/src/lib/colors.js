export const CHANNEL_COLORS = [
  '#4FC3F7', '#81C784', '#FFB74D', '#E57373',
  '#BA68C8', '#4DB6AC', '#F06292', '#AED581',
  '#FFD54F', '#64B5F6', '#A1887F', '#90A4AE',
];

export function channelLabel(i) {
  return `CH ${String(i).padStart(2, '0')}`;
}

export function channelKey(i) {
  return `s_${String(i).padStart(2, '0')}`;
}
