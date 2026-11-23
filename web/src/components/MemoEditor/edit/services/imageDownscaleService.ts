/**
 * Fork: a phone photo is 4-8 MB, far more than a note needs, and the full file downloads every time the
 * note opens. Photos are scaled down in the browser before upload, so the stored file is the small one.
 */

/** The longest edge kept. 2048px is sharp on a full-width retina phone screen. */
export const MAX_IMAGE_EDGE = 2048;
/** Images this small and this narrow are uploaded as they are. */
const SKIP_BELOW_BYTES = 1_500_000;
const JPEG_QUALITY = 0.85;
const ELIGIBLE_TYPES = new Set(["image/jpeg", "image/png", "image/heic", "image/heif"]);
/** Markers of an Android motion photo, whose JPEG carries an MP4 that re-encoding would drop. */
const MOTION_PHOTO_MARKER = /MotionPhoto|MicroVideo|GCamera:/;
const MOTION_PHOTO_PROBE_BYTES = 256 * 1024;

export interface PreparedImage {
  file: File;
  width: number;
  height: number;
}

const isAndroidMotionPhoto = async (file: File): Promise<boolean> => {
  const head = new Uint8Array(await file.slice(0, MOTION_PHOTO_PROBE_BYTES).arrayBuffer());
  return MOTION_PHOTO_MARKER.test(new TextDecoder("latin1").decode(head));
};

const canvasToBlob = (canvas: HTMLCanvasElement, type: string): Promise<Blob | null> =>
  new Promise((resolve) => canvas.toBlob(resolve, type, JPEG_QUALITY));

const renamedFor = (name: string, type: string): string => {
  const wanted = type === "image/png" ? ".png" : ".jpg";
  return /\.(jpe?g|png|heic|heif)$/i.test(name) ? name.replace(/\.[^.]+$/, wanted) : `${name}${wanted}`;
};

export const imageDownscaleService = {
  /**
   * The file scaled to fit MAX_IMAGE_EDGE, or null when the original should upload: not a photo this can
   * handle, already small, a motion photo, undecodable here, or no smaller once re-encoded.
   */
  async prepare(file: File): Promise<PreparedImage | null> {
    if (!ELIGIBLE_TYPES.has(file.type) || typeof createImageBitmap !== "function") return null;
    try {
      if (await isAndroidMotionPhoto(file)) return null;
      // Applies the EXIF orientation, so the pixels are upright and the tag is not needed after.
      const bitmap = await createImageBitmap(file);
      try {
        const longEdge = Math.max(bitmap.width, bitmap.height);
        if (longEdge <= MAX_IMAGE_EDGE && file.size <= SKIP_BELOW_BYTES) return null;
        const scale = Math.min(1, MAX_IMAGE_EDGE / longEdge);
        const width = Math.max(1, Math.round(bitmap.width * scale));
        const height = Math.max(1, Math.round(bitmap.height * scale));
        const canvas = document.createElement("canvas");
        canvas.width = width;
        canvas.height = height;
        const context = canvas.getContext("2d");
        if (!context) return null;
        context.drawImage(bitmap, 0, 0, width, height);
        // PNG stays PNG so transparency survives; everything else becomes JPEG.
        const type = file.type === "image/png" ? "image/png" : "image/jpeg";
        const blob = await canvasToBlob(canvas, type);
        if (!blob || blob.size >= file.size) return null;
        return { file: new File([blob], renamedFor(file.name, type), { type, lastModified: file.lastModified }), width, height };
      } finally {
        bitmap.close();
      }
    } catch {
      // Never block an upload on an optimization.
      return null;
    }
  },
};
