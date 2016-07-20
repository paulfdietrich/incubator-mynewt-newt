/**
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/* this file maintains a list of all the symbols from a */

package builder

import (
	"fmt"
	"strings"

	"mynewt.apache.org/newt/util"
)

type SymbolInfo struct {
	bpkg    string
	name    string
	code    string
	section string
	ext     string
	size    int
	loc     int
}

type SymbolMap map[string]SymbolInfo

func NewSymbolMap() *SymbolMap {
	val := &SymbolMap{}
	return val
}

func NewSymbolInfo() *SymbolInfo {
	val := &SymbolInfo{}
	return val
}

func (s *SymbolMap) Add(info SymbolInfo) {
	(*s)[info.name] = info
}

func IdenticalUnion(s1 *SymbolMap, s2 *SymbolMap) *SymbolMap {
	s3 := NewSymbolMap()
	/* look through all symbols in S1 and if they are in s1,
	 * add to new map s3 */
	for name, info1 := range *s1 {
		if info2, ok := (*s2)[name]; ok {
			/* compare to info 1 */
			if info1.code == info2.code &&
				info1.size == info2.size && info1.bpkg == info2.bpkg {
				s3.Add(info1)
			}
		}
	}
	return s3
}

type SymbolMapIterator func(s *SymbolInfo)

func (s *SymbolMap) Iterate(iter SymbolMapIterator) {
	for _, info1 := range *s {
		iter(&info1)
	}
}

func dumpSi(si *SymbolInfo) {
	fmt.Printf("  %s(%s) (%s) -- (%s) %d (%d) from %s\n",
		(*si).name, (*si).ext, (*si).code, (*si).section,
		(*si).size, (*si).loc, (*si).bpkg)
}

func (si *SymbolInfo) Dump() {
	dumpSi(si)
}

func (si *SymbolInfo) IsLocal() bool {
	val := (*si).code[:1]

	if val == "l" {
		return true
	}
	return false
}

func (si *SymbolInfo) IsWeak() bool {
	val := (*si).code[1:2]

	if val == "w" {
		return true
	}
	return false
}

func (si *SymbolInfo) IsDebug() bool {
	val := (*si).code[5:6]

	if val == "d" {
		return true
	}
	return false
}

func (si *SymbolInfo) IsSection(section string) bool {
	val := (*si).section
	return strings.HasPrefix(val, section)
}

func (si *SymbolInfo) IsFile() bool {
	val := (*si).code[6:7]

	if val == "f" {
		return true
	}
	return false
}

func (s *SymbolMap) Dump() {
	s.Iterate(dumpSi)
}

// Merge - merges given maps into 1 map
// values will be overridden by last matching key - value
func (s1 *SymbolMap) Merge(s2 *SymbolMap) *SymbolMap {

	for k, v := range *s2 {

		if val, ok := (*s1)[k]; ok {
			/* We already have this in the MAP */
			if val.IsWeak() && !v.IsWeak() {
				(*s1)[k] = v
			} else if v.IsWeak() && !val.IsWeak() {
				/* nothing to do here as this is OK not to replace */
			} else if v.IsLocal() && val.IsLocal() {
				/* two locals that must conflict with name */
				/* have to have separate instances of these */
				util.StatusMessage(util.VERBOSITY_VERBOSE,
					"Local Symbol Conflict: %s from packages %s and %s \n",
					v.name, v.bpkg, val.bpkg)
				(*s2).Remove(k)
			} else {
				util.StatusMessage(util.VERBOSITY_QUIET,
					"Global Symbol Conflict: %s from packages %s and %s \n",
					v.name, v.bpkg, val.bpkg)
			}
		} else {
			(*s1)[k] = v
		}

	}
	return s1
}

func (s *SymbolMap) Remove(name string) {
	delete(*s, name)
}

/* Returns true if the symbol is present in the symbol map */
func (s *SymbolMap) Find(name string) (*SymbolInfo, bool) {
	val, ok := (*s)[name]
	return &val, ok
}
