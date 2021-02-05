/*
Copyright 2021 Gravitational, Inc.

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

package client

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"io"
	"os"

	"github.com/gravitational/trace"
)

// IdentityFile represents the basic components of an identity file.
type IdentityFile struct {
	// PrivateKey is a PEM encoded key
	PrivateKey []byte
	// Certs contains PEM encoded certificates
	Certs struct {
		// SSH is a cert used for SSH
		SSH []byte
		// TLS is a cert used for TLS
		TLS []byte
	}
	// CACerts contains PEM encoded CA certificates
	CACerts struct {
		// SSH are CA certs used for SSH
		SSH [][]byte
		// TLS are CA certs used for TLS
		TLS [][]byte
	}
}

// TLS returns the identity file's associated TLS config.
func (idf *IdentityFile) TLS() (*tls.Config, error) {
	cert, err := tls.X509KeyPair(idf.Certs.TLS, idf.PrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pool := x509.NewCertPool()
	for _, caCerts := range idf.CACerts.TLS {
		if !pool.AppendCertsFromPEM(caCerts) {
			return nil, trace.BadParameter("invalid CA cert PEM")
		}
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	}, nil
}

// DecodeIdentity attempts to break up the contents of an identity file into its
// respective components. An IdentityFile
func DecodeIdentity(idFile io.Reader) (*IdentityFile, error) {
	scanner := bufio.NewScanner(idFile)
	var ident IdentityFile
	// Subslice of scanner's buffer pointing to current line
	// with leading and trailing whitespace trimmed.
	var line []byte
	// Attempt to scan to the next line.
	scanln := func() bool {
		if !scanner.Scan() {
			line = nil
			return false
		}
		line = bytes.TrimSpace(scanner.Bytes())
		return true
	}
	// Check if the current line starts with prefix `p`.
	hasPrefix := func(p string) bool {
		return bytes.HasPrefix(line, []byte(p))
	}
	// Get an "owned" copy of the current line.
	cloneln := func() []byte {
		ln := make([]byte, len(line))
		copy(ln, line)
		return ln
	}
	// Scan through all lines of identity file.  Lines with a known prefix
	// are copied out of the scanner's buffer.  All others are ignored.
	for scanln() {
		switch {
		case hasPrefix("ssh"):
			ident.Certs.SSH = cloneln()
		case hasPrefix("@cert-authority"):
			ident.CACerts.SSH = append(ident.CACerts.SSH, cloneln())
		case hasPrefix("-----BEGIN"):
			// Current line marks the beginning of a PEM block.  Consume all
			// lines until a corresponding END is found.
			var pemBlock []byte
			for {
				pemBlock = append(pemBlock, line...)
				pemBlock = append(pemBlock, '\n')
				if hasPrefix("-----END") {
					break
				}
				if !scanln() {
					// If scanner has terminated in the middle of a PEM block, either
					// the reader encountered an error, or the PEM block is a fragment.
					if err := scanner.Err(); err != nil {
						return nil, trace.Wrap(err)
					}
					return nil, trace.BadParameter("invalid PEM block (fragment)")
				}
			}
			// Decide where to place the pem block based on
			// which pem blocks have already been found.
			switch {
			case ident.PrivateKey == nil:
				ident.PrivateKey = pemBlock
			case ident.Certs.TLS == nil:
				ident.Certs.TLS = pemBlock
			default:
				ident.CACerts.TLS = append(ident.CACerts.TLS, pemBlock)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &ident, nil
}

// IdentityFileFromPath attempts to retrieve an IdentityFile from the specified path.
func IdentityFileFromPath(path string) (*IdentityFile, error) {
	r, err := os.Open(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer r.Close()
	return DecodeIdentity(r)
}
