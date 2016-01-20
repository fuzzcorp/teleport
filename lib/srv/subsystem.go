/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package srv

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

type subsys struct {
	Name string
}

type subsystem interface {
	execute(*ssh.ServerConn, ssh.Channel, *ssh.Request, *ctx) error
}

func parseSubsystemRequest(srv *Server, req *ssh.Request) (subsystem, error) {
	var s subsys
	if err := ssh.Unmarshal(req.Payload, &s); err != nil {
		return nil, fmt.Errorf("failed to parse subsystem request, error: %v", err)
	}
	if strings.HasPrefix(s.Name, "tun:") {
		return parseTunSubsys(s.Name)
	}
	if strings.HasPrefix(s.Name, "mux:") {
		return parseMuxSubsys(s.Name)
	}
	if strings.HasPrefix(s.Name, "join:") {
		return parseJoinSubsys(s.Name, srv)
	}
	if strings.HasPrefix(s.Name, "ls:") {
		return parseLSSubsys(s.Name)
	}
	if srv.proxyMode && strings.HasPrefix(s.Name, "proxy:") {
		return parseProxySubsys(s.Name, srv)
	}
	if srv.proxyMode && strings.HasPrefix(s.Name, "proxysites") {
		return parseProxySitesSubsys(s.Name, srv)
	}
	return nil, fmt.Errorf("unrecognized subsystem: %v", s.Name)
}
