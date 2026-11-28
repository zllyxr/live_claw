export const profileImageSizes = {
  avatar: { width: 512, height: 512 },
  background: { width: 1200, height: 675 }
};

export type ProfileImageKind = keyof typeof profileImageSizes;

export function cropBounds(imageWidth: number, imageHeight: number, kind: ProfileImageKind,
  zoom: number, centerX: number, centerY: number) {
  const output = profileImageSizes[kind];
  const scale = Math.max(output.width / imageWidth, output.height / imageHeight) * zoom;
  const width = Math.min(imageWidth, output.width / scale);
  const height = Math.min(imageHeight, output.height / scale);
  return {
    x: Math.max(0, Math.min(imageWidth - width, centerX - width / 2)),
    y: Math.max(0, Math.min(imageHeight - height, centerY - height / 2)),
    width, height
  };
}
