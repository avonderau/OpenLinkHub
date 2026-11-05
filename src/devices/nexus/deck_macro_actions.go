package nexus

import (
 "context"
 "errors"
 "fmt"
 "strconv"
 "time"

 "OpenLinkHub/src/macro"
)

// ValidateDeckSavedMacroID is shared with the Nexus touchscreen. A Deck
// references existing macro IDs only: it never stores or executes freeform
// command sequences inside a key configuration.
func ValidateDeckSavedMacroID(id string)bool {
 return validSharedMacroID(id)
}

// RunDeckSavedMacro executes one immutable snapshot of an existing validated
// OpenLinkHub macro, with the same duration/action bounds used by Nexus.
// The caller supplies cancellation (for example Deck device shutdown).
func RunDeckSavedMacro(ctx context.Context,id string)error{
 if !validSharedMacroID(id){return errors.New("invalid saved macro ID")}
 number,_:=strconv.Atoi(id)
 profile,exists:=macro.SnapshotProfile(number)
 if !exists{return fmt.Errorf("saved macro %s not found",id)}
 if !validateSharedMacro(profile){return fmt.Errorf("saved macro %s exceeds Nexus safety limits",id)}
 if ctx==nil{return errors.New("missing macro cancellation context")}
 bound,cancel:=context.WithTimeout(ctx,nexusSharedMacroMaxDuration)
 defer cancel()
 if !runSavedMacro(bound,profile){
  if err:=bound.Err();err!=nil{return fmt.Errorf("saved macro interrupted: %w",err)}
  return errors.New("saved macro action failed")
 }
 return nil
}

// DeckMacroMaxDuration documents the pre-existing execution ceiling for
// button macros without exporting internal macro scheduler constants.
func DeckMacroMaxDuration()time.Duration{return nexusSharedMacroMaxDuration}
