package nexus

import (
 "context"
 "errors"
 "fmt"
 "net/url"
 "os"
 "os/exec"
 "regexp"
 "strings"
 "time"

 "OpenLinkHub/src/config"
 "OpenLinkHub/src/logger"
)

// Applications launch by desktop file ID through gtk-launch; URLs open through
// xdg-open. User-controlled strings never become shell commands.
var desktopEntryID=regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func validateNexusURL(raw string)error{
 if len(raw)<10||len(raw)>256||strings.TrimSpace(raw)!=raw{return errors.New("URL must be between 10 and 256 characters")}
 for _,r:=range raw{if r<0x21||r==0x7f{return errors.New("URL contains whitespace or control characters")}}
 u,err:=url.Parse(raw)
 if err!=nil{return fmt.Errorf("invalid URL: %w",err)}
 if (u.Scheme!="http"&&u.Scheme!="https")||u.Opaque!=""||u.Host==""||u.Hostname()==""||u.User!=nil{
  return errors.New("only http/https URLs without embedded credentials are supported")
 }
 if strings.Contains(u.Host,"\\"){return errors.New("invalid URL hostname")}
 return nil
}

func validateNexusDesktopID(id string)error{
 if !desktopEntryID.MatchString(id)||strings.Contains(id,"..")||strings.HasSuffix(id,".") {
  return errors.New("invalid desktop application ID")
 }
 return nil
}

func validateNexusLaunchAction(action,target string)error{
 switch action{
 case "url":return validateNexusURL(target)
 case "application":return validateNexusDesktopID(target)
 default:return errors.New("invalid launch action")
 }
}

// launchPermissionCheck is kept separate from process execution so privilege
// and session preconditions are testable without actually starting programs.
func launchPermissionCheck(uid int,display,wayland string)error{
 if uid==0{return errors.New("desktop launching is disabled when OpenLinkHub runs as root")}
 if display==""&&wayland==""{return errors.New("desktop launching requires a graphical user session")}
 return nil
}

func nexusDirectDesktopAllowed(systemService bool,uid int,display,wayland string)bool {
 return !systemService && launchPermissionCheck(uid,display,wayland)==nil
}

func runNexusLaunchAction(action,target string)error{
 if err:=validateNexusLaunchAction(action,target);err!=nil{return err}
 if err:=launchPermissionCheck(os.Geteuid(),os.Getenv("DISPLAY"),os.Getenv("WAYLAND_DISPLAY"));err!=nil{return err}
 program:="xdg-open"
 if action=="application"{program="gtk-launch"}
 ctx,cancel:=context.WithTimeout(context.Background(),4*time.Second)
 defer cancel()
 command:=exec.CommandContext(ctx,program,target)
 command.Stdin=nil
 if err:=command.Run();err!=nil{
  if errors.Is(ctx.Err(),context.DeadlineExceeded){return errors.New("desktop launcher timed out")}
  return fmt.Errorf("%s launch failed: %w",action,err)
 }
 return nil
}

// Prevent repeated touches from creating unlimited launcher processes. The
// HID reader must not block while a desktop application is starting.
var launchSlot=make(chan struct{},1)
// dispatchNexusLaunchAction first tries the current process's graphical
// session. In headless system services it forwards to the opt-in user desktop
// agent instead. No commands are launched as root.
func dispatchNexusLaunchAction(action,target string)error{
 if err:=validateNexusLaunchAction(action,target);err!=nil{return err}
 // A background service may inherit DISPLAY from a shell but still has
 // no right to use the logged-in user's graphical session. In that case,
 // deliver directly to the opt-in user agent instead of trying xdg-open
 // as the service account and silently failing.
 if nexusDirectDesktopAllowed(config.IsSystemService(),os.Geteuid(),os.Getenv("DISPLAY"),os.Getenv("WAYLAND_DISPLAY")){
  if err:=runNexusLaunchAction(action,target);err==nil{return nil}
 }
 return forwardNexusDesktopLaunch(action,target)
}

func executeNexusLaunchAction(action,target string){
 if err:=validateNexusLaunchAction(action,target);err!=nil{
  logger.Log(logger.Fields{"action":action,"error":err.Error()}).Warn("Nexus launch action rejected")
  return
 }
 select{case launchSlot<-struct{}{}:default:return}
 go func(){
  defer func(){<-launchSlot}()
  // This is a physical touch path. Do not expose the launcher as an HTTP
  // command endpoint. Log only the action category, never the private URL.
  if err:=dispatchNexusLaunchAction(action,target);err!=nil{
   logger.Log(logger.Fields{"action":action,"error":err.Error()}).Warn("Nexus desktop launch failed")
  }
 }()
}
