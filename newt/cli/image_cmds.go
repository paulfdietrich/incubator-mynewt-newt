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

func createImageRunCmd(cmd *cobra.Command, args []string) {
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

	app_image, err := image.NewImage(b.App)
	if err != nil {
		NewtUsage(cmd, err)
		return
	}

	err = app_image.SetVersion(args[1])
	if err != nil {
		NewtUsage(cmd, err)
	}

	if len(args) > 2 {
		var keyId uint8 = 0
		if len(args) > 3 {
			keyId64, err := strconv.ParseUint(args[3], 10, 8)
			if err != nil {
				NewtUsage(cmd,
					util.NewNewtError("Key ID must be between 0-255"))
			}
			keyId = uint8(keyId64)
		}
		err = app_image.SetSigningKey(args[2], keyId)
		if err != nil {
			NewtUsage(cmd, err)
		}
	}

	err = app_image.Generate()
	if err != nil {
		NewtUsage(cmd, err)
	}

	err = app_image.CreateManifest(t)
	if err != nil {
		NewtUsage(cmd, err)
	}
	util.StatusMessage(util.VERBOSITY_DEFAULT,
		"App image succesfully generated: %s\n", app_image.TargetImg())
	util.StatusMessage(util.VERBOSITY_DEFAULT, "Build manifest: %s\n",
		app_image.ManifestFile())

	loader_image, err := image.NewImage(b.Loader)
	if err != nil {
		NewtUsage(cmd, err)
		return
	}

	err = loader_image.SetVersion(args[1])
	if err != nil {
		NewtUsage(cmd, err)
	}

	if len(args) > 2 {
		var keyId uint8 = 0
		if len(args) > 3 {
			keyId64, err := strconv.ParseUint(args[3], 10, 8)
			if err != nil {
				NewtUsage(cmd,
					util.NewNewtError("Key ID must be between 0-255"))
			}
			keyId = uint8(keyId64)
		}
		err = loader_image.SetSigningKey(args[2], keyId)
		if err != nil {
			NewtUsage(cmd, err)
		}
	}

	err = loader_image.Generate()
	if err != nil {
		NewtUsage(cmd, err)
	}

	err = loader_image.CreateManifest(t)
	if err != nil {
		NewtUsage(cmd, err)
	}
	util.StatusMessage(util.VERBOSITY_DEFAULT,
		"Loader image succesfully generated: %s\n", loader_image.TargetImg())
	util.StatusMessage(util.VERBOSITY_DEFAULT, "Build manifest: %s\n",
		loader_image.ManifestFile())

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
