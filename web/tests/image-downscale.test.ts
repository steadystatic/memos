import { afterEach, describe, expect, it, vi } from "vitest";
import { imageDownscaleService, MAX_IMAGE_EDGE } from "@/components/MemoEditor/edit/services/imageDownscaleService";

const photo = (bytes: number, type = "image/jpeg", name = "IMG_1.jpeg", head = "") => {
  const data = new Uint8Array(bytes);
  data.set(new TextEncoder().encode(head));
  return new File([data], name, { type });
};

const stubDecoding = (width: number, height: number, outputBytes: number) => {
  vi.stubGlobal(
    "createImageBitmap",
    vi.fn(async () => ({ width, height, close: vi.fn() })),
  );
  const canvas = {
    width: 0,
    height: 0,
    getContext: () => ({ drawImage: vi.fn() }),
    toBlob: (callback: (blob: Blob) => void, type: string) => callback(new Blob([new Uint8Array(outputBytes)], { type })),
  };
  vi.spyOn(document, "createElement").mockReturnValue(canvas as unknown as HTMLElement);
  return canvas;
};

describe("imageDownscaleService", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("scales a large photo down to the longest-edge limit", async () => {
    const canvas = stubDecoding(4032, 3024, 700_000);
    const prepared = await imageDownscaleService.prepare(photo(6_000_000));
    expect(prepared?.width).toBe(MAX_IMAGE_EDGE);
    expect(prepared?.height).toBe(Math.round((3024 * MAX_IMAGE_EDGE) / 4032));
    expect(canvas.width).toBe(MAX_IMAGE_EDGE);
    expect(prepared?.file.size).toBe(700_000);
    expect(prepared?.file.type).toBe("image/jpeg");
  });

  it("turns a HEIC photo into a .jpg", async () => {
    stubDecoding(4032, 3024, 600_000);
    const prepared = await imageDownscaleService.prepare(photo(5_000_000, "image/heic", "IMG_2.HEIC"));
    expect(prepared?.file.name).toBe("IMG_2.jpg");
    expect(prepared?.file.type).toBe("image/jpeg");
  });

  it("leaves a small image alone", async () => {
    stubDecoding(1200, 800, 100_000);
    expect(await imageDownscaleService.prepare(photo(400_000))).toBeNull();
  });

  it("leaves an Android motion photo alone, whose JPEG carries a video", async () => {
    stubDecoding(4032, 3024, 600_000);
    expect(await imageDownscaleService.prepare(photo(6_000_000, "image/jpeg", "MVIMG.jpg", "<x:xmpmeta GCamera:MicroVideo=1"))).toBeNull();
  });

  it("keeps the original when re-encoding does not make it smaller", async () => {
    stubDecoding(4032, 3024, 7_000_000);
    expect(await imageDownscaleService.prepare(photo(6_000_000))).toBeNull();
  });

  it("ignores types it cannot handle and falls back when decoding fails", async () => {
    expect(await imageDownscaleService.prepare(photo(6_000_000, "image/gif", "a.gif"))).toBeNull();
    vi.stubGlobal(
      "createImageBitmap",
      vi.fn(async () => Promise.reject(new Error("undecodable"))),
    );
    expect(await imageDownscaleService.prepare(photo(6_000_000))).toBeNull();
  });
});
