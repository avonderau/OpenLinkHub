package devices

import (
 "sync"
 "testing"
 "time"
)

func TestInitialDeviceRegistrationWaitsForCorsairConstructors(t *testing.T) {
 var wg sync.WaitGroup
 wg.Add(1)
 done := make(chan bool, 1)
 go func(){done <- waitForInitialDeviceRegistrations(&wg, 500*time.Millisecond)}()
 select{
 case <-done:t.Fatal("device initialization returned before Corsair constructor")
 case <-time.After(15*time.Millisecond):
 }
 wg.Done()
 select{
 case ok:= <-done:
  if !ok{t.Fatal("completed initialization reported a timeout")}
 case <-time.After(time.Second):t.Fatal("finished initialization did not unblock startup")
 }
}

func TestInitialDeviceRegistrationIsBounded(t *testing.T) {
 var wg sync.WaitGroup
 wg.Add(1)
 if waitForInitialDeviceRegistrations(&wg, 10*time.Millisecond) {
  t.Fatal("unresponsive device should not hold WebUI startup indefinitely")
 }
 wg.Done()
}
