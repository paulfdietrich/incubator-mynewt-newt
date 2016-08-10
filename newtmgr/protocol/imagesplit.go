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

package protocol

import (
	"encoding/json"
	"fmt"
	"strings"

	"mynewt.apache.org/newt/util"
)

type SplitMode int

const (
	NONE SplitMode = iota
	TEST
	RUN
)

var splitMode = [...]string{
	"none",
	"test",
	"run",
}

/* is the enum valid */
func (sm SplitMode) Valid() bool {
	for val, _ := range splitMode {
		if int(sm) == val {
			return true
		}
	}
	return false
}

/* returns the enum as a string */
func (sm SplitMode) String() string {
	if sm > RUN || sm < 0 {
		return "Invalid!"
	}
	return splitMode[sm]
}

/* parses the enum from a string */
func ParseSplitMode(str string) (SplitMode, error) {
	for val, key := range splitMode {
		if strings.EqualFold(key, str) {
			return SplitMode(val), nil
		}
	}
	return NONE, util.NewNewtError("Invalid value for Split Mode %v" + str)
}

type Split struct {
	Split      SplitMode `json:"splitMode"`
	ReturnCode int       `json:"rc"`
}

func NewSplit() (*Split, error) {
	s := &Split{}
	return s, nil
}

func (s *Split) EncoderReadRequest() (*NmgrReq, error) {
	msg := "{}"

	data := []byte(msg)

	nmr, err := NewNmgrReq()
	if err != nil {
		return nil, err
	}

	nmr.Op = NMGR_OP_READ
	nmr.Flags = 0
	nmr.Group = NMGR_GROUP_ID_IMAGE
	nmr.Id = IMGMGR_NMGR_OP_SPLITAPP
	nmr.Len = uint16(len(data))
	nmr.Data = data

	return nmr, nil
}

func (s *Split) EncoderWriteRequest() (*NmgrReq, error) {

	data, err := json.Marshal(s)

	fmt.Println(string(data[:]))

	if err != nil {
		return nil, err
	}

	nmr, err := NewNmgrReq()
	if err != nil {
		return nil, err
	}

	nmr.Op = NMGR_OP_WRITE
	nmr.Flags = 0
	nmr.Group = NMGR_GROUP_ID_IMAGE
	nmr.Id = IMGMGR_NMGR_OP_SPLITAPP
	nmr.Len = uint16(len(data))
	nmr.Data = data

	return nmr, nil
}

func DecodeSplitReadResponse(data []byte) (*Split, error) {
	i := &Split{}

	if len(data) == 0 {
		return i, nil
	}

	err := json.Unmarshal(data, &i)
	if err != nil {
		return nil, util.NewNewtError(fmt.Sprintf("Invalid incoming json: %s",
			err.Error()))
	}
	if i.ReturnCode != 0 {
		return nil, util.NewNewtError(fmt.Sprintf("Target error: %d",
			i.ReturnCode))
	}
	return i, nil
}
