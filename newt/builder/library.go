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

/* this file parses a library file for the build and returns
 * a list of all the symbols with their types and sizes */

/* gets an objdump -t and parses into a symbolMap" */

package builder

import (
	"bytes"
	"regexp"
	"strconv"

	"mynewt.apache.org/newt/util"
)

/* This is a tricky thing to parse. Right now, I keep all the
 * flags together and just store the offset, size, name and flags.
* 00012970 l       .bss	00000000 _end
* 00011c60 l       .init_array	00000000 __init_array_start
* 00011c60 l       .init_array	00000000 __preinit_array_start
* 000084b0 g     F .text	00000034 os_arch_start
* 00000000 g       .debug_aranges	00000000 __HeapBase
* 00011c88 g     O .data	00000008 g_os_task_list
* 000082cc g     F .text	0000004c os_idle_task
* 000094e0 g     F .text	0000002e .hidden __gnu_uldivmod_helper
* 00000000 g       .svc_table	00000000 SVC_Count
* 000125e4 g     O .bss	00000004 g_console_is_init
* 00009514 g     F .text	0000029c .hidden __divdi3
* 000085a8 g     F .text	00000054 os_eventq_put
*/
func ParseObjectLine(line string, r *regexp.Regexp) (error, *SymbolInfo) {

	answer := r.FindAllStringSubmatch(line, 11)

	if len(answer) == 0 {
		return nil, nil
	}

	data := answer[0]

	if len(data) != 6 {
		return nil, nil
	}

	si := NewSymbolInfo()

	si.name = data[5]

	v, err := strconv.ParseUint(data[1], 16, 32)

	if err != nil {
		return nil, nil
	}

	si.loc = int(v)

	v, err = strconv.ParseUint(data[4], 16, 32)

	if err != nil {
		return nil, nil
	}

	si.size = int(v)
	si.code = data[2]

	return nil, si
}

func (b *Builder) RemoveSymbol(si *SymbolInfo) error {
	c, err := b.target.NewCompiler(b.AppElfPath())

	if err != nil {
		return err
	}

	libraryFile := b.ArchivePath(si.bpkg)
	cmd := c.RemoveSymbolCmd(si.name, libraryFile)

	_, err = util.ShellCommand(cmd)
	return err
}

func (b *Builder) ParseObjectLibrary(bp *BuildPackage) (error, *SymbolMap) {

	file := b.ArchivePath(bp.Name())

	c, err := b.target.NewCompiler(b.AppElfPath())

	if err != nil {
		return err, nil
	}

	err, out := c.ParseLibrary(file)

	if err != nil {
		return err, nil
	}

	sm := NewSymbolMap()

	buffer := bytes.NewBuffer(out)

	r, err := regexp.Compile("^([0-9A-Fa-f]+)[\t ]+([lgu! ][w ][C ][W ][Ii ][Dd ][FfO ])[\t ]+([^\t\n\f\r ]+)[\t ]+([0-9a-fA-F]+)[\t ]([^\t\n\f\r ]+)")

	if err != nil {
		return err, nil
	}

	for {
		line, err := buffer.ReadString('\n')
		if err != nil {
			break
		}
		err, si := ParseObjectLine(line, r)

		if err == nil && si != nil {
			if (*si).size != 0 {
				/* assign the library and add to the list */
				(*si).bpkg = bp.Name()
				sm.Add(*si)
			}
		}
	}

	return nil, sm
}
