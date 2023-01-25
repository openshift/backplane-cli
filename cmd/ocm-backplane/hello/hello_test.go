package hello

import (
  "testing"
  "github.com/spf13/cobra"

)

func TestHello(t *testing.T){
  got := runHello(&cobra.Command{},[]string{})
  want := true
  if want!=got {
		t.Errorf("Expected %t, got %t",want,got)
	}
}
