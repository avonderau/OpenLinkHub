package deck

// Model definitions for the first Deck Editor hardware families. The model
// registry is intentionally keyed by BOTH USB vendor and product ID so that
// third-party controllers cannot collide with Corsair's PID-only registry.
type Model struct {
 Name string
 Family string
 VendorID uint16
 ProductID uint16
 Rows int
 Columns int
 Protocol int // Mirabox protocol version; zero for Elgato (different protocol)
 KeyPixels int
}

var models = []Model{
 {Name:"Mirabox HSV293S", Family:"mirabox", VendorID:0x5548, ProductID:0x6670, Rows:3, Columns:6, Protocol:1, KeyPixels:85},
 {Name:"Mirabox HSV293SV3", Family:"mirabox", VendorID:0x6603, ProductID:0x1014, Rows:3, Columns:6, Protocol:3, KeyPixels:95},
 {Name:"Mirabox HSV293SV3", Family:"mirabox", VendorID:0x6603, ProductID:0x1005, Rows:3, Columns:6, Protocol:3, KeyPixels:95},
 {Name:"Ajazz AKP153", Family:"mirabox", VendorID:0x5548, ProductID:0x6674, Rows:3, Columns:6, Protocol:1, KeyPixels:85},
 {Name:"Elgato Stream Deck Original", Family:"elgato", VendorID:0x0fd9, ProductID:0x0060, Rows:3, Columns:5, KeyPixels:72},
 {Name:"Elgato Stream Deck Original V2", Family:"elgato", VendorID:0x0fd9, ProductID:0x006d, Rows:3, Columns:5, KeyPixels:72},
 {Name:"Elgato Stream Deck Mini", Family:"elgato", VendorID:0x0fd9, ProductID:0x0063, Rows:2, Columns:3, KeyPixels:80},
 {Name:"Elgato Stream Deck Mini Mk2", Family:"elgato", VendorID:0x0fd9, ProductID:0x0090, Rows:2, Columns:3, KeyPixels:80},
 {Name:"Elgato Stream Deck MK.2", Family:"elgato", VendorID:0x0fd9, ProductID:0x0080, Rows:3, Columns:5, KeyPixels:72},
 {Name:"Elgato Stream Deck XL", Family:"elgato", VendorID:0x0fd9, ProductID:0x006c, Rows:4, Columns:8, KeyPixels:96},
 {Name:"Elgato Stream Deck XL V2", Family:"elgato", VendorID:0x0fd9, ProductID:0x008f, Rows:4, Columns:8, KeyPixels:96},
}

// SupportedModels exposes the same authoritative registry used for HID
// discovery so that the Settings list includes connected and disconnected models.
func SupportedModels() []Model {
 out:=make([]Model,len(models))
 copy(out,models)
 return out
}

func LookupModel(vendorID, productID uint16) (Model, bool) {
 for _, m := range models {
  if m.VendorID == vendorID && m.ProductID == productID { return m,true }
 }
 return Model{},false
}

func SupportedVendors() []uint16 { return []uint16{0x5548,0x6603,0x0fd9} }
func (m Model) Keys() int { return m.Rows*m.Columns }
func (m Model) NativeImagePixels(key int) int {
 if m.Protocol == 3 && (key+1)%6 == 0 { return 82 }
 return m.KeyPixels
}
func (m Model) SupportsKeyRelease() bool { return m.Protocol == 3 || m.Family == "elgato" }
