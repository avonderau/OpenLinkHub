package nexus

import "image"

// The Nexus LCD controller consumes B,G,R,A bytes. The upstream legacy
// renderer swaps red and blue in its drawing colors before the shared
// renderImageToBytes encoder. Custom screens render ordinary RGBA so their
// channel swap must happen here, only on the hardware transfer path.
// This intentionally leaves browser-side previews and other LCD modes alone.
func encodeCustomNexusLCDFrame(frame *image.RGBA) []byte {
	data := renderImageToBytes(frame)
	for offset := 0; offset+3 < len(data); offset += 4 {
		data[offset], data[offset+2] = data[offset+2], data[offset]
	}
	return data
}
