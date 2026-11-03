package nexus

import "image"

// applyNexusSoftwareBrightness dims the final RGB framebuffer without using
// undocumented HID backlight commands. A missing setting (0) means 100%,
// preserving every existing saved configuration.
func applyNexusSoftwareBrightness(frame *image.RGBA, brightness int) {
 if frame==nil || brightness==0 || brightness==100 {return}
 if brightness<20 || brightness>100 {return} // caller validates layout
 for i:=0;i+3<len(frame.Pix);i+=4 {
  frame.Pix[i]=uint8(uint16(frame.Pix[i])*uint16(brightness)/100)
  frame.Pix[i+1]=uint8(uint16(frame.Pix[i+1])*uint16(brightness)/100)
  frame.Pix[i+2]=uint8(uint16(frame.Pix[i+2])*uint16(brightness)/100)
 }
}
