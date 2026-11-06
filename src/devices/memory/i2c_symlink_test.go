package memory

import (
 "os"
 "path/filepath"
 "testing"
)

func TestGetI2cSensorResolvesSymlink(t *testing.T) {
 dir:=t.TempDir()
 real:=filepath.Join(dir,"i2c-7")
 if err:=os.WriteFile(real,nil,0600);err!=nil{t.Fatal(err)}
 link:=filepath.Join(dir,"i2c-dimm")
 if err:=os.Symlink(real,link);err!=nil{t.Fatal(err)}
 device:=&Device{Path:link}
 if got:=device.getI2cSensor();got!=7{
  t.Fatalf("i2c device symlink %q resolved as %d, want 7",link,got)
 }
}

func TestGetI2cSensorDirectDevice(t *testing.T) {
 device:=&Device{Path:"/dev/i2c-3"}
 if got:=device.getI2cSensor();got!=3{
  t.Fatalf("direct device name resolved as %d, want 3",got)
 }
}
