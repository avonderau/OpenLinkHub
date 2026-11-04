package main

import (
 "OpenLinkHub/src/controller"
 "OpenLinkHub/src/devices/nexus"
 "context"
 "fmt"
 "os"
 "os/signal"
 "syscall"
)

// waitForExit listens for a program termination and switches the device back to hardware mode
func waitForExit() {
	terminateSignals := make(chan os.Signal, 1)
	signal.Notify(terminateSignals, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM)
	for {
		select {
		case <-terminateSignals:
			controller.Stop() // Back to hardware mode
			os.Exit(0)
		}
	}
}

// main entry point
func main() {
 // Desktop-agent mode runs as the logged-in user, without starting the
 // device daemon or WebUI. It is intentionally opt-in and local-only.
 if len(os.Args)>1 && os.Args[1]=="--nexus-desktop-agent" {
  socketPath:=""
  if len(os.Args)==4 && os.Args[2]=="--socket" {socketPath=os.Args[3]} else if len(os.Args)!=2 {
   fmt.Fprintln(os.Stderr,"Usage: OpenLinkHub --nexus-desktop-agent [--socket /path/to/desktop-launch.sock]")
   os.Exit(2)
  }
  ctx,cancel:=signal.NotifyContext(context.Background(),os.Interrupt,syscall.SIGTERM)
  defer cancel()
  if err:=nexus.RunNexusDesktopLaunchAgent(ctx,socketPath);err!=nil{
   fmt.Fprintln(os.Stderr,err)
   os.Exit(1)
  }
  return
 }
 go waitForExit()
 controller.Start()
}
