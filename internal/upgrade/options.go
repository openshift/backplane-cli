package upgrade

import (
	"io"

	"github.com/sirupsen/logrus"
)

type WithLog struct{ Log logrus.FieldLogger }

func (w WithLog) ConfigureCmd(c *CmdConfig) {
	c.Log = w.Log
}

func (w WithLog) ConfigureSafeWriter(c *SafeWriterConfig) {
	c.Log = w.Log
}

type WithOut struct{ Out io.Writer }

func (w WithOut) ConfigureCmd(c *CmdConfig) {
	c.Out = w.Out
}

type WithWriter struct{ Writer Writer }

func (w WithWriter) ConfigureCmd(c *CmdConfig) {
	c.Writer = w.Writer
}

type WithBinaryName string

func (w WithBinaryName) ConfigureCmd(c *CmdConfig) {
	c.BinaryName = string(w)
}

type WithOrg string

func (w WithOrg) ConfigureCmd(c *CmdConfig) {
	c.Org = string(w)
}

type WithRepo string

func (w WithRepo) ConfigureCmd(c *CmdConfig) {
	c.Repo = string(w)
}
