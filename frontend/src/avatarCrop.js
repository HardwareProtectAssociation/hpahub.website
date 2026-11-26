export function cropRect(width, height, zoom, x, y) {
  const side = Math.min(width, height) / Number(zoom);
  return { side, left: (width - side) * Number(x) / 100, top: (height - side) * Number(y) / 100 };
}
