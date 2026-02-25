// Package server provides Sakura server related operations
package server

import (
	"fmt"
)

type ServersCommand struct {
	Commands map[string]command
}

// CLIにコマンドを登録
func Compile() (*ServersCommand, error) {
	cmd := &ServersCommand{}
	cmd.Commands = make(map[string]command)

	cmd.Commands["restart"] = &restartCommand{}
	cmd.Commands["info"] = &hostsCommand{}

	return cmd, nil
}

func (sc *ServersCommand) Execute(args []string) error {
	verb := args[0]
	c, ok := sc.Commands[verb]
	if !ok {
		return fmt.Errorf("unknown command: `%s`", verb)
	}
	return c.Execute(args[1:])
}

type command interface {
	Execute(args []string) error
}
