package deck

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"github.com/sstallion/go-hid"
)

type fakeElgatoSession struct {
	fakeDeckSession
	features [][]byte
}

func (s *fakeElgatoSession) SendFeatureReport(data []byte) (int, error) {
	s.features = append(s.features, append([]byte(nil), data...))
	return len(data), nil
}

func elgatoModel(t *testing.T, pid uint16) Model {
	t.Helper()
	m, ok := LookupModel(0x0fd9, pid)
	if !ok {
		t.Fatalf("missing Elgato model %04x", pid)
	}
	return m
}

func TestElgatoGenerationsCoverEveryRegisteredModel(t *testing.T) {
	for _, m := range SupportedModels() {
		if m.Family == "elgato" && elgatoGenerationFor(m) == elgatoGenUnknown {
			t.Errorf("%s (%04x) has no protocol generation", m.Name, m.ProductID)
		}
	}
	if elgatoGenerationFor(Model{Family: "mirabox", ProductID: 0x0080}) != elgatoGenUnknown {
		t.Fatal("Mirabox model treated as Elgato")
	}
}

func TestElgatoResetAndBrightnessFeatureReports(t *testing.T) {
	cases := []struct {
		pid        uint16
		reset      []byte
		bright     []byte
		reportSize int
	}{
		{0x0060, []byte{0x0b, 0x63}, []byte{0x05, 0x55, 0xaa, 0xd1, 0x01, 70}, 17},
		{0x0063, []byte{0x0b, 0x63}, []byte{0x05, 0x55, 0xaa, 0xd1, 0x01, 70}, 17},
		{0x0080, []byte{0x03, 0x02}, []byte{0x03, 0x08, 70}, 32},
		{0x008f, []byte{0x03, 0x02}, []byte{0x03, 0x08, 70}, 32},
	}
	for _, c := range cases {
		m := elgatoModel(t, c.pid)
		s := new(fakeElgatoSession)
		if err := ElgatoReset(s, m); err != nil {
			t.Fatal(err)
		}
		if err := ElgatoBrightness(s, m, 70); err != nil {
			t.Fatal(err)
		}
		if len(s.features) != 2 {
			t.Fatalf("%04x: %d feature reports", c.pid, len(s.features))
		}
		for i, want := range [][]byte{c.reset, c.bright} {
			got := s.features[i]
			if len(got) != c.reportSize || !bytes.HasPrefix(got, want) {
				t.Errorf("%04x report %d = % x", c.pid, i, got[:8])
			}
		}
	}
	if err := ElgatoBrightness(new(fakeElgatoSession), elgatoModel(t, 0x0080), 101); err == nil {
		t.Fatal("brightness above 100 accepted")
	}
	if err := ElgatoReset(new(fakeDeckSession), elgatoModel(t, 0x0080)); err == nil {
		t.Fatal("session without feature reports accepted")
	}
}

func TestElgatoV2ImagePagesUseEightByteHeaders(t *testing.T) {
	m := elgatoModel(t, 0x0080)
	data := bytes.Repeat([]byte{0xab}, 2500)
	s := new(fakeElgatoSession)
	if err := ElgatoWriteKeyImage(s, m, 7, data); err != nil {
		t.Fatal(err)
	}
	if len(s.reports) != 3 {
		t.Fatalf("expected 3 pages, got %d", len(s.reports))
	}
	lengths := []int{1016, 1016, 468}
	for page, report := range s.reports {
		if len(report) != 1024 {
			t.Fatalf("page %d length %d", page, len(report))
		}
		last := byte(0)
		if page == 2 {
			last = 1
		}
		want := []byte{0x02, 0x07, 7, last, byte(lengths[page]), byte(lengths[page] >> 8), byte(page), 0}
		if !bytes.Equal(report[:8], want) {
			t.Errorf("page %d header % x, want % x", page, report[:8], want)
		}
	}
}

func TestElgatoOriginalSplitsImageInTwoAndFlipsColumns(t *testing.T) {
	m := elgatoModel(t, 0x0060)
	frame, err := ElgatoKeyImage(m, 0, quadrantImage(72))
	if err != nil {
		t.Fatal(err)
	}
	if len(frame) != 54+72*72*3 || string(frame[:2]) != "BM" {
		t.Fatalf("unexpected BMP size %d", len(frame))
	}
	s := new(fakeElgatoSession)
	if err := ElgatoWriteKeyImage(s, m, 0, frame); err != nil {
		t.Fatal(err)
	}
	if len(s.reports) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(s.reports))
	}
	for page, report := range s.reports {
		if len(report) != 8191 {
			t.Fatalf("page %d length %d", page, len(report))
		}
		// Logical key 0 (top left) is device key 4 on the Original.
		want := []byte{0x02, 0x01, byte(page + 1), 0, byte(page), 5}
		if !bytes.Equal(report[:6], want) {
			t.Errorf("page %d header % x, want % x", page, report[:6], want)
		}
	}
}

func TestElgatoMiniPagesStartAtZero(t *testing.T) {
	m := elgatoModel(t, 0x0063)
	frame, err := ElgatoKeyImage(m, 5, quadrantImage(80))
	if err != nil {
		t.Fatal(err)
	}
	s := new(fakeElgatoSession)
	if err := ElgatoWriteKeyImage(s, m, 5, frame); err != nil {
		t.Fatal(err)
	}
	if want := (len(frame) + 1007) / 1008; len(s.reports) != want {
		t.Fatalf("expected %d pages, got %d", want, len(s.reports))
	}
	first, last := s.reports[0], s.reports[len(s.reports)-1]
	if !bytes.Equal(first[:6], []byte{0x02, 0x01, 0, 0, 0, 6}) || last[4] != 1 {
		t.Fatalf("unexpected headers % x / % x", first[:6], last[:6])
	}
}

// bmpPixel reads one pixel back from encodeElgatoBMP's bottom-up layout.
func bmpPixel(data []byte, size, x, y int) color.RGBA {
	stride := (size*3 + 3) &^ 3
	at := 54 + (size-1-y)*stride + x*3
	return color.RGBA{R: data[at+2], G: data[at+1], B: data[at], A: 255}
}

func quadrantImage(size int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			c := color.RGBA{A: 255}
			if x < size/2 && y < size/2 {
				c.R = 255 // top left red
			}
			if x >= size/2 && y < size/2 {
				c.G = 255 // top right green
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func TestElgatoImageOrientation(t *testing.T) {
	// Original mirrors both axes: top-left red lands bottom-right.
	m := elgatoModel(t, 0x0060)
	data, err := ElgatoKeyImage(m, 0, quadrantImage(72))
	if err != nil {
		t.Fatal(err)
	}
	if p := bmpPixel(data, 72, 70, 70); p.R < 200 || p.G > 50 {
		t.Fatalf("Original bottom-right = %+v, want red", p)
	}
	if binary.LittleEndian.Uint32(data[22:]) != 72 {
		t.Fatal("BMP must be bottom-up (positive height)")
	}
	// Mini transposes: top-right green lands bottom-left.
	m = elgatoModel(t, 0x0063)
	data, err = ElgatoKeyImage(m, 0, quadrantImage(80))
	if err != nil {
		t.Fatal(err)
	}
	if p := bmpPixel(data, 80, 2, 78); p.G < 200 || p.R > 50 {
		t.Fatalf("Mini bottom-left = %+v, want green", p)
	}
	if p := bmpPixel(data, 80, 2, 2); p.R < 200 {
		t.Fatalf("Mini top-left = %+v, want red", p)
	}
	// MK.2 is JPEG at 72 px, mirrored both axes.
	m = elgatoModel(t, 0x0080)
	data, err = ElgatoKeyImage(m, 0, quadrantImage(144))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() != 72 {
		t.Fatalf("JPEG size %v", decoded.Bounds())
	}
	if r, g, _, _ := decoded.At(68, 68).RGBA(); r>>8 < 200 || g>>8 > 60 {
		t.Fatalf("MK.2 bottom-right not red: %d %d", r>>8, g>>8)
	}
}

func TestElgatoKeyStates(t *testing.T) {
	mk2 := elgatoModel(t, 0x0080)
	report := make([]byte, 4+15)
	report[0] = 0x01
	report[4+6] = 1
	states, ok := ElgatoKeyStates(mk2, report)
	if !ok || !states[6] || states[5] {
		t.Fatalf("MK.2 states %v ok=%v", states, ok)
	}
	original := elgatoModel(t, 0x0060)
	report = make([]byte, 1+15)
	report[0] = 0x01
	report[1+4] = 1 // device key 4 is logical top-left
	states, ok = ElgatoKeyStates(original, report)
	if !ok || !states[0] || states[4] {
		t.Fatalf("Original states %v ok=%v", states, ok)
	}
	if _, ok := ElgatoKeyStates(mk2, []byte{0x01, 0, 0}); ok {
		t.Fatal("short report accepted")
	}
	if _, ok := ElgatoKeyStates(mk2, append([]byte{0x02}, make([]byte, 30)...)); ok {
		t.Fatal("non-input report accepted")
	}
}

func TestElgatoPressesFireOnceUntilReleased(t *testing.T) {
	m := elgatoModel(t, 0x0080)
	down := make([]byte, 4+15)
	down[0], down[4+2] = 0x01, 1
	up := make([]byte, 4+15)
	up[0] = 0x01
	presses, state := hardwareKeyPresses(m, down, nil)
	if len(presses) != 1 || presses[0] != 2 {
		t.Fatalf("first press %v", presses)
	}
	if presses, state = hardwareKeyPresses(m, down, state); len(presses) != 0 {
		t.Fatalf("held key repeated %v", presses)
	}
	if presses, state = hardwareKeyPresses(m, up, state); len(presses) != 0 {
		t.Fatalf("release fired %v", presses)
	}
	if presses, _ = hardwareKeyPresses(m, down, state); len(presses) != 1 {
		t.Fatalf("second press %v", presses)
	}
}

func TestElgatoDeviceAttachDrawsEveryKey(t *testing.T) {
	m := elgatoModel(t, 0x0080)
	d := &Device{
		Serial: deviceID(&hid.DeviceInfo{VendorID: m.VendorID, ProductID: m.ProductID, SerialNbr: "AL1"}, m),
		Model:  m, root: t.TempDir(), layout: DefaultLayout(m), done: make(chan struct{}), pressedKey: -1,
	}
	d.layout.Brightness = 80
	s := new(fakeElgatoSession)
	if err := d.attachSession(s); err != nil {
		t.Fatal(err)
	}
	status := d.GetStatus()
	if !status.Connected || !status.KeyOutputSupported {
		t.Fatalf("status %+v", status)
	}
	if len(s.features) != 3 || s.features[2][2] != 80 {
		t.Fatalf("expected reset, startup and saved brightness, got %d reports", len(s.features))
	}
	keys := map[byte]bool{}
	for _, report := range s.reports {
		if report[0] == 0x02 && report[1] == 0x07 && report[3] == 1 {
			keys[report[2]] = true
		}
	}
	if len(keys) != m.Keys() {
		t.Fatalf("drew %d of %d keys", len(keys), m.Keys())
	}
	for _, report := range s.reports {
		if bytes.Contains(report[:16], []byte("CRT")) {
			t.Fatal("Mirabox command sent to an Elgato device")
		}
	}
}

func TestElgatoReconnectIgnoresUsagePage(t *testing.T) {
	m := elgatoModel(t, 0x0080)
	info := &hid.DeviceInfo{VendorID: m.VendorID, ProductID: m.ProductID, Path: "/dev/hidraw7", SerialNbr: "AL1", UsagePage: 0x0c, Usage: 1}
	d := &Device{Serial: deviceID(info, m), Model: m}
	if !d.matchesReconnectCandidate(info) {
		t.Fatal("Elgato replacement rejected")
	}
	other := *info
	other.SerialNbr = "AL2"
	if d.matchesReconnectCandidate(&other) {
		t.Fatal("different Stream Deck accepted")
	}
}
