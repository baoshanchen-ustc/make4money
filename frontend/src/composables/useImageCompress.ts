/**
 * Image compression utility using Canvas API.
 * Outputs WebP format with JPEG fallback.
 */

export function useImageCompress() {
  /**
   * Compress an image file to a data URL.
   * @param file - Source image file
   * @param maxWidth - Maximum width in pixels (default 1200)
   * @param maxSizeKB - Maximum output size in KB (default 500)
   * @returns Base64 data URL string
   */
  async function compressImage(
    file: File,
    maxWidth = 1200,
    maxSizeKB = 500,
    maxHeight = 900
  ): Promise<string> {
    return new Promise((resolve, reject) => {
      const reader = new FileReader()
      reader.onerror = () => reject(new Error('Failed to read file'))
      reader.onload = () => {
        const img = new Image()
        img.onerror = () => reject(new Error('Failed to load image'))
        img.onload = () => {
          try {
            const result = compressWithCanvas(img, maxWidth, maxSizeKB, maxHeight)
            resolve(result)
          } catch (err) {
            reject(err)
          }
        }
        img.src = reader.result as string
      }
      reader.readAsDataURL(file)
    })
  }

  function compressWithCanvas(
    img: HTMLImageElement,
    maxWidth: number,
    maxSizeKB: number,
    maxHeight: number
  ): string {
    let { width, height } = img

    // Scale down if needed (respect both width and height limits)
    if (width > maxWidth) {
      height = Math.round((height * maxWidth) / width)
      width = maxWidth
    }
    if (height > maxHeight) {
      width = Math.round((width * maxHeight) / height)
      height = maxHeight
    }

    const canvas = document.createElement('canvas')
    canvas.width = width
    canvas.height = height

    const ctx = canvas.getContext('2d')
    if (!ctx) throw new Error('Canvas 2D context not available')

    ctx.drawImage(img, 0, 0, width, height)

    // Try WebP first
    const supportsWebP = canvas.toDataURL('image/webp').startsWith('data:image/webp')
    const format = supportsWebP ? 'image/webp' : 'image/jpeg'

    // Start at quality 0.85 and reduce until under maxSizeKB
    let quality = 0.85
    let dataUrl = canvas.toDataURL(format, quality)

    while (estimateSizeKB(dataUrl) > maxSizeKB && quality > 0.2) {
      quality -= 0.1
      dataUrl = canvas.toDataURL(format, quality)
    }

    return dataUrl
  }

  function estimateSizeKB(dataUrl: string): number {
    // Base64 payload = everything after "data:...;base64,"
    const base64 = dataUrl.split(',')[1] || ''
    // Base64 encodes 3 bytes as 4 chars
    return (base64.length * 3) / 4 / 1024
  }

  return { compressImage }
}
