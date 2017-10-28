/*
Copyright 2017 Smile SA.

Parts of code come from peer-finder, and "utils" to have "sets" structures
from Kubernetes project.

Copyright 2014 The Kubernetes Authors.

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

// A small utility program to lookup hostnames of endpoints in a service.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"./utils/sets"
)

const (
	pollPeriod = 1 * time.Second
)

var (
	svc       = flag.String("service", "", "Governing service responsible for the DNS records of the domain this pod is in.")
	namespace = flag.String("ns", "", "The namespace this pod is running in. If unspecified, the POD_NAMESPACE env var is used.")
	domain    = flag.String("domain", "", "The Cluster Domain which is used by the Cluster, if not set tries to determine it from /etc/resolv.conf file.")
	rack      = flag.String("rack", "", "The rack name for dynomite")
	dc        = flag.String("dc", "", "The datacenter name for dynomite")
	token     = flag.String("token", "", "token prefix")
	tokenreg  = regexp.MustCompile(`^.+-(\d+)`)
	allPeers  = make([]string, 0, 128)
	locker    = &sync.Mutex{}
	myName    string
)

func lookup(svcName string) (sets.String, error) {
	endpoints := sets.NewString()
	_, srvRecords, err := net.LookupSRV("", "", svcName)
	if err != nil {
		return endpoints, err
	}
	for _, srvRecord := range srvRecords {
		// The SRV records ends in a "." for the root domain
		ep := fmt.Sprintf("%v", srvRecord.Target[:len(srvRecord.Target)-1])
		endpoints.Insert(ep)
	}
	return endpoints, nil
}

func main() {
	flag.Parse()

	if t := os.Getenv("DYN_TOKEN"); t != "" {
		*token = t
	}

	if r := os.Getenv("DYN_RACK"); r != "" {
		*rack = r
	}

	if d := os.Getenv("DYN_DC"); d != "" {
		*dc = d
	}

	if s := os.Getenv("DYN_SVC"); s != "" {
		*svc = s
	}

	// That handle seve a florida like response for dynomite florida provider
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		locker.Lock()
		defer locker.Unlock()
		n := len(allPeers)
		for i, p := range allPeers {
			// do not include this host in list
			if p == myName {
				continue
			}
			// find "id" in the name, eg. rep-0.domain.svc.local => id is "0"
			// so the token will be token + "0", and so on
			b := strings.Split(p, ".")[0]
			if matches := tokenreg.FindAllStringSubmatch(b, 1); len(matches) > 0 && len(matches[0]) > 0 {
				id := matches[0][1]
				t := *token + id
				entry := fmt.Sprintf("%s:%d:%s:%s:%s", p, 8101, *rack, *dc, t)
				w.Write([]byte(entry))
				if i < n-1 {
					w.Write([]byte{'|'})
				}
			} else {
				log.Println(matches)
				log.Println("The domain name " + p + " cannot be used to find id to generate" +
					"token, please check that name is like name-0, name-1...")
				continue
			}

		}
	})

	go looping()
	http.ListenAndServe(":8080", nil)
}

func looping() {

	ns := *namespace
	if ns == "" {
		ns = os.Getenv("POD_NAMESPACE")
	}
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Failed to get hostname: %s", err)
	}
	var domainName string

	// If domain is not provided, try to get it from resolv.conf
	if *domain == "" {
		resolvConfBytes, err := ioutil.ReadFile("/etc/resolv.conf")
		resolvConf := string(resolvConfBytes)
		if err != nil {
			log.Fatal("Unable to read /etc/resolv.conf")
		}

		var re *regexp.Regexp
		if ns == "" {
			// Looking for a domain that looks like with *.svc.**
			re, err = regexp.Compile(`\A(.*\n)*search\s{1,}(.*\s{1,})*(?P<goal>[a-zA-Z0-9-]{1,63}.svc.([a-zA-Z0-9-]{1,63}\.)*[a-zA-Z0-9]{2,63})`)
		} else {
			// Looking for a domain that looks like svc.**
			re, err = regexp.Compile(`\A(.*\n)*search\s{1,}(.*\s{1,})*(?P<goal>svc.([a-zA-Z0-9-]{1,63}\.)*[a-zA-Z0-9]{2,63})`)
		}
		if err != nil {
			log.Fatalf("Failed to create regular expression: %v", err)
		}

		groupNames := re.SubexpNames()
		result := re.FindStringSubmatch(resolvConf)
		for k, v := range result {
			if groupNames[k] == "goal" {
				if ns == "" {
					// Domain is complete if ns is empty
					domainName = v
				} else {
					// Need to convert svc.** into ns.svc.**
					domainName = ns + "." + v
				}
				break
			}
		}
		log.Printf("Determined Domain to be %s", domainName)

	} else {
		domainName = strings.Join([]string{ns, "svc", *domain}, ".")
	}

	if *svc == "" || domainName == "" {
		log.Fatalf("Incomplete args, require -service or an env var for DYN_SERVICE and -ns or an env var for POD_NAMESPACE.")
	}

	myName = strings.Join([]string{hostname, *svc, domainName}, ".")
	for newPeers, peers := sets.NewString(), sets.NewString(); ; time.Sleep(pollPeriod) {
		newPeers, err = lookup(*svc)
		if err != nil {
			log.Printf("%v", err)
			continue
		}

		if newPeers.Equal(peers) {
			// no new peer
			continue
		}

		if !newPeers.Has(myName) {
			log.Printf("Have not found myself in list yet.\nMy Hostname: %s\nHosts in list: %s", myName, strings.Join(newPeers.List(), ", "))
			continue
		}
		peerList := newPeers.List()
		sort.Strings(peerList)
		locker.Lock()
		allPeers = peerList
		locker.Unlock()

		log.Printf("Peer list updated\nwas %v\nnow %v", peers.List(), newPeers.List())
		peers = newPeers
	}
	log.Printf("Peer finder exiting")
}
