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

package cli

import (
	"strconv"

	"github.com/spf13/cobra"
	"mynewt.apache.org/newt/newt/builder"
	"mynewt.apache.org/newt/newt/image"
	"mynewt.apache.org/newt/newt/project"
	"mynewt.apache.org/newt/util"
)

func CreateImage(b *builder.Builder, version string,
	keystr string, keyId uint8, bootable bool) error {

	/* do the app image */
	app_image, err := image.NewImage(b, bootable)
	if err != nil {
		return err
	}

	err = app_image.SetVersion(version)
	if err != nil {
		return err
	}

	if keystr != "" {
		err = app_image.SetSigningKey(keystr, keyId)
		if err != nil {
			return err
		}
	}

	err = app_image.Generate()
	if err != nil {
		return err
	}

	err = app_image.CreateManifest(b.GetTarget())
	if err != nil {
		return err
	}
	util.StatusMessage(util.VERBOSITY_DEFAULT,
		"App image succesfully generated: %s\n", app_image.TargetImg())
	util.StatusMessage(util.VERBOSITY_DEFAULT, "Build manifest: %s\n",
		app_image.ManifestFile())

	return nil
}

func createImageRunCmd(cmd *cobra.Command, args []string) {
	var keyId uint8
	var keystr string

	if err := project.Initialize(); err != nil {
		NewtUsage(cmd, err)
	}
	if len(args) < 2 {
		NewtUsage(cmd, util.NewNewtError("Must specify target and version"))
	}

	targetName := args[0]
	t := ResolveTarget(targetName)
	if t == nil {
		NewtUsage(cmd, util.NewNewtError("Invalid target name: "+targetName))
	}

	version := args[1]

	if len(args) > 2 {
		if len(args) > 3 {
			keyId64, err := strconv.ParseUint(args[3], 10, 8)
			if err != nil {
				NewtUsage(cmd,
					util.NewNewtError("Key ID must be between 0-255"))
			}
			keyId = uint8(keyId64)
		}
		keystr = args[2]
	}

	b, err := builder.NewTargetBuilder(t)
	if err != nil {
		NewtUsage(cmd, err)
		return
	}

	err = b.PrepBuild()
	if err != nil {
		NewtUsage(cmd, err)
		return
	}

	if b.Loader == nil {
		err = CreateImage(b.App, version, keystr, keyId, true)
		if err != nil {
			NewtUsage(cmd, err)
			return
		}
	} else {
		err = CreateImage(b.App, version, keystr, keyId, false)
		if err != nil {
			NewtUsage(cmd, err)
			return
		}
		err = CreateImage(b.Loader, version, keystr, keyId, true)
		if err != nil {
			NewtUsage(cmd, err)
			return
		}
	}
}

func AddImageCommands(cmd *cobra.Command) {
	createImageHelpText := "Create image by adding image header to created " +
		"binary file for <target-name>. Version number in the header is set " +
		"to be <version>.\n\nTo sign the image give private key as <signing_key>."
	createImageHelpEx := "  newt create-image <target-name> <version>\n"
	createImageHelpEx += "  newt create-image my_target1 1.2.0\n"
	createImageHelpEx += "  newt create-image my_target1 1.2.0.3\n"
	createImageHelpEx += "  newt create-image my_target1 1.2.0.3 private.pem\n"

	createImageCmd := &cobra.Command{
		Use:     "create-image",
		Short:   "Add image header to target binary",
		Long:    createImageHelpText,
		Example: createImageHelpEx,
		Run:     createImageRunCmd,
	}
	cmd.AddCommand(createImageCmd)
}
