package deck

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"

	"golang.org/x/image/draw"
)

// The Elgato Stream Deck wire format is written in Go from the reference in
// OpenActionAPI/rust-elgato-streamdeck (MPL-2.0), the HID library used by
// OpenDeck. No source was copied. Three hardware generations exist:
//
//   - Original (0x0060): BMP keys, 8191-byte image reports split in two
//     halves, physical columns reported right-to-left.
//   - Mini / Mini Mk.2: BMP keys rotated 90 degrees, 1024-byte reports.
//   - Original V2, MK.2 and XL: JPEG keys, 1024-byte reports, 8-byte header.
//
// Stream Deck+, Neo, Pedal and the module variants are out of scope.
type elgatoGeneration int

const (
	elgatoGenUnknown elgatoGeneration = iota
	elgatoGenOriginal
	elgatoGenMini
	elgatoGenV2
)

// featureReporter is implemented by *hid.Device. Elgato uses HID feature
// reports for reset and brightness; Mirabox uses output reports only.
type featureReporter interface {
	SendFeatureReport([]byte) (int, error)
}

var errNotElgato = errors.New("not a supported Elgato Stream Deck")

func elgatoGenerationFor(m Model) elgatoGeneration {
	if m.Family != "elgato" {
		return elgatoGenUnknown
	}
	switch m.ProductID {
	case 0x0060:
		return elgatoGenOriginal
	case 0x0063, 0x0090:
		return elgatoGenMini
	case 0x006d, 0x0080, 0x00a5, 0x006c, 0x008f:
		return elgatoGenV2
	}
	return elgatoGenUnknown
}

func sendElgatoFeature(session any, report []byte) error {
	reporter, ok := session.(featureReporter)
	if !ok {
		return errors.New("HID session does not support feature reports")
	}
	n, err := reporter.SendFeatureReport(report)
	if err != nil {
		return err
	}
	if n != len(report) {
		return io.ErrShortWrite
	}
	return nil
}

// ElgatoReset clears the key images and returns the device to its idle state.
func ElgatoReset(session any, m Model) error {
	switch elgatoGenerationFor(m) {
	case elgatoGenOriginal, elgatoGenMini:
		report := make([]byte, 17)
		report[0], report[1] = 0x0b, 0x63
		return sendElgatoFeature(session, report)
	case elgatoGenV2:
		report := make([]byte, 32)
		report[0], report[1] = 0x03, 0x02
		return sendElgatoFeature(session, report)
	}
	return errNotElgato
}

// ElgatoBrightness sets the LCD backlight from 0 to 100 percent.
func ElgatoBrightness(session any, m Model, percent uint8) error {
	if percent > 100 {
		return errors.New("brightness must be 0-100")
	}
	switch elgatoGenerationFor(m) {
	case elgatoGenOriginal, elgatoGenMini:
		report := make([]byte, 17)
		copy(report, []byte{0x05, 0x55, 0xaa, 0xd1, 0x01, percent})
		return sendElgatoFeature(session, report)
	case elgatoGenV2:
		report := make([]byte, 32)
		copy(report, []byte{0x03, 0x08, percent})
		return sendElgatoFeature(session, report)
	}
	return errNotElgato
}

// elgatoFlipKey mirrors a row-major key index horizontally. The Original
// reports and addresses its columns right-to-left.
func elgatoFlipKey(m Model, key int) int {
	col := key % m.Columns
	return key - col + (m.Columns - 1 - col)
}

// ElgatoKeyImage converts a rendered key to the device's native encoding:
// resized, rotated and mirrored as the panel is mounted, then BMP or JPEG.
func ElgatoKeyImage(m Model, key int, src image.Image) ([]byte, error) {
	gen := elgatoGenerationFor(m)
	if gen == elgatoGenUnknown {
		return nil, errNotElgato
	}
	if key < 0 || key >= m.Keys() {
		return nil, errInvalidKey
	}
	if src == nil {
		return nil, errors.New("missing key image")
	}
	size := m.KeyPixels
	resized := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(resized, resized.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)
	draw.CatmullRom.Scale(resized, resized.Bounds(), src, src.Bounds(), draw.Over, nil)
	native := image.NewRGBA(resized.Bounds())
	last := size - 1
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var sx, sy int
			if gen == elgatoGenMini {
				// Rotating 90 degrees clockwise and then mirroring
				// horizontally is a transpose of the square image.
				sx, sy = y, x
			} else {
				// Mirror both axes (a 180-degree turn).
				sx, sy = last-x, last-y
			}
			native.Set(x, y, resized.At(sx, sy))
		}
	}
	if gen == elgatoGenV2 {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, native, &jpeg.Options{Quality: 90}); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	return encodeElgatoBMP(native), nil
}

// encodeElgatoBMP writes an uncompressed 24-bit bottom-up BMP, the format
// the first-generation devices decode.
func encodeElgatoBMP(img *image.RGBA) []byte {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	stride := (w*3 + 3) &^ 3
	pixels := stride * h
	out := make([]byte, 54+pixels)
	copy(out, "BM")
	binary.LittleEndian.PutUint32(out[2:], uint32(len(out)))
	binary.LittleEndian.PutUint32(out[10:], 54)
	binary.LittleEndian.PutUint32(out[14:], 40)
	binary.LittleEndian.PutUint32(out[18:], uint32(w))
	binary.LittleEndian.PutUint32(out[22:], uint32(h))
	binary.LittleEndian.PutUint16(out[26:], 1)
	binary.LittleEndian.PutUint16(out[28:], 24)
	binary.LittleEndian.PutUint32(out[34:], uint32(pixels))
	binary.LittleEndian.PutUint32(out[38:], 3780)
	binary.LittleEndian.PutUint32(out[42:], 3780)
	for y := 0; y < h; y++ {
		row := out[54+(h-1-y)*stride:]
		for x := 0; x < w; x++ {
			p := img.RGBAAt(x, y)
			row[x*3], row[x*3+1], row[x*3+2] = p.B, p.G, p.R
		}
	}
	return out
}

// ElgatoWriteKeyImage sends one encoded key image. Elgato devices show
// each image as soon as its last page arrives; there is no separate flush.
func ElgatoWriteKeyImage(writer outputReporter, m Model, key int, data []byte) error {
	gen := elgatoGenerationFor(m)
	if gen == elgatoGenUnknown {
		return errNotElgato
	}
	if key < 0 || key >= m.Keys() {
		return errInvalidKey
	}
	if len(data) == 0 || len(data) > 1<<20 {
		return errors.New("invalid key image data")
	}
	reportLength, headerLength := 1024, 8
	payloadLength := reportLength - headerLength
	switch gen {
	case elgatoGenOriginal:
		reportLength, headerLength = 8191, 16
		payloadLength = (len(data) + 1) / 2
		if payloadLength > reportLength-headerLength {
			return errors.New("key image too large for Stream Deck Original")
		}
		key = elgatoFlipKey(m, key)
	case elgatoGenMini:
		headerLength = 16
		payloadLength = reportLength - headerLength
	}
	for page, sent := 0, 0; sent < len(data); page++ {
		length := len(data) - sent
		if length > payloadLength {
			length = payloadLength
		}
		lastPage := byte(0)
		if sent+length == len(data) {
			lastPage = 1
		}
		report := make([]byte, reportLength)
		switch gen {
		case elgatoGenOriginal:
			copy(report, []byte{0x02, 0x01, byte(page + 1), 0, lastPage, byte(key + 1)})
		case elgatoGenMini:
			copy(report, []byte{0x02, 0x01, byte(page), 0, lastPage, byte(key + 1)})
		default:
			copy(report, []byte{0x02, 0x07, byte(key), lastPage,
				byte(length), byte(length >> 8), byte(page), byte(page >> 8)})
		}
		copy(report[headerLength:], data[sent:sent+length])
		n, err := writer.Write(report)
		if err != nil {
			return fmt.Errorf("image page %d: %w", page, err)
		}
		if n != len(report) {
			return io.ErrShortWrite
		}
		sent += length
	}
	return nil
}

// ElgatoKeyStates decodes an input report into one pressed flag per key, in
// the editor's row-major order. Elgato reports the whole key state each time.
func ElgatoKeyStates(m Model, report []byte) ([]bool, bool) {
	gen := elgatoGenerationFor(m)
	if gen == elgatoGenUnknown || len(report) == 0 || report[0] != 0x01 {
		return nil, false
	}
	offset := 4
	if gen == elgatoGenOriginal || gen == elgatoGenMini {
		offset = 1
	}
	if len(report) < offset+m.Keys() {
		return nil, false
	}
	states := make([]bool, m.Keys())
	for i := range states {
		source := i
		if gen == elgatoGenOriginal {
			source = elgatoFlipKey(m, i)
		}
		states[i] = report[offset+source] != 0
	}
	return states, true
}
