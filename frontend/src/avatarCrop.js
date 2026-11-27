export function cropRect(width, height, zoom, x, y) {
  const side = Math.min(width, height) / Number(zoom);
  return { side, left: (width - side) * Number(x) / 100, top: (height - side) * Number(y) / 100 };
}

export function zoomCropPosition(width, height, zoom, x, y, nextZoom) {
  const current = cropRect(width, height, zoom, x, y);
  const nextSide = Math.min(width, height) / Number(nextZoom);
  const maxLeft = Math.max(0, width - nextSide);
  const maxTop = Math.max(0, height - nextSide);
  const clampPercent = (offset, maxOffset) => maxOffset === 0 ? 50 : Math.max(0, Math.min(100, offset / maxOffset * 100));
  return {
    x: clampPercent(current.left + current.side / 2 - nextSide / 2, maxLeft),
    y: clampPercent(current.top + current.side / 2 - nextSide / 2, maxTop),
  };
}
