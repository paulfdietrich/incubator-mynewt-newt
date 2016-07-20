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

package builder

import (
	"mynewt.apache.org/newt/newt/interfaces"
	"mynewt.apache.org/newt/newt/pkg"
	"mynewt.apache.org/newt/newt/project"
	"mynewt.apache.org/newt/newt/target"
	"mynewt.apache.org/newt/newt/toolchain"
	"mynewt.apache.org/newt/util"
)

type TargetBuilder struct {
	compilerPkg *pkg.LocalPackage
	Bsp         *pkg.BspPackage
	target      *target.Target

	App     *Builder
	AppList interfaces.PackageList

	Loader     *Builder
	LoaderList interfaces.PackageList
}

func NewTargetBuilder(target *target.Target) (*TargetBuilder, error) {
	t := &TargetBuilder{}

	/* TODO */
	t.target = target

	return t, nil
}

func (t *TargetBuilder) NewCompiler(dstDir string) (*toolchain.Compiler, error) {
	c, err := toolchain.NewCompiler(t.compilerPkg.BasePath(), dstDir,
		t.target.BuildProfile)

	return c, err
}

func (t *TargetBuilder) PrepBuild() error {
	if t.Bsp != nil {
		// Already prepped
		return nil
	}

	// Collect the seed packages.
	bspPkg := t.target.Bsp()
	if bspPkg == nil {
		if t.target.BspName == "" {
			return util.NewNewtError("BSP package not specified by target")
		} else {
			return util.NewNewtError("BSP package not found: " +
				t.target.BspName)
		}
	}

	t.Bsp = pkg.NewBspPackage(bspPkg)
	compilerPkg := t.resolveCompiler()
	if compilerPkg == nil {
		if t.Bsp.CompilerName == "" {
			return util.NewNewtError("Compiler package not specified by BSP")
		} else {
			return util.NewNewtError("Compiler package not found: " +
				t.Bsp.CompilerName)
		}
	}

	appPkg := t.target.App()

	loaderPkg := t.target.Loader()

	targetPkg := t.target.Package()

	t.compilerPkg = compilerPkg

	app, err := NewBuilder(t, "app")

	if err == nil {
		t.App = app
	} else {
		return err
	}

	loader, err := NewBuilder(t, "loader")

	if err == nil {
		t.Loader = loader
	} else {
		return err
	}

	t.Loader.PrepBuild(loaderPkg, bspPkg, targetPkg)
	t.LoaderList = project.ResetDeps(nil)

	t.App.PrepBuild(appPkg, bspPkg, targetPkg)
	t.AppList = project.ResetDeps(nil)

	return nil
}

func (t *TargetBuilder) Build() error {
	var err error

	if err = t.target.Validate(true); err != nil {
		return err
	}

	if err = t.PrepBuild(); err != nil {
		return err
	}

	if err = t.Bsp.Reload(t.Loader.Features()); err != nil {
		return err
	}

	loader_sm := NewSymbolMap()

	if t.Loader != nil {

		project.ResetDeps(t.LoaderList)
		err = t.Loader.Build()

		if err != nil {
			return err
		}

		err = t.Loader.Link(t.Bsp.LinkerScript)

		if err != nil {
			return err
		}

		util.StatusMessage(util.VERBOSITY_DEFAULT,
			"Generating Loader Symbol Map\n")
		err, loader_sm = t.Loader.FetchSymbolMap()
		if err != nil {
			return err
		}
	}

	if err := t.Bsp.Reload(t.App.Features()); err != nil {
		return err
	}

	/* Build the Apps */
	project.ResetDeps(t.AppList)
	err = t.App.Build()

	if err != nil {
		return err
	}

	util.StatusMessage(util.VERBOSITY_DEFAULT,
		"Generating Application Symbol Map\n")
	err, app_sm := t.App.FetchSymbolMap()
	if err != nil {
		return err
	}

	util.StatusMessage(util.VERBOSITY_DEFAULT,
		"Merging Symbol Maps\n")
	union_sm := IdenticalUnion(loader_sm, app_sm)

	/* handle special symbols */
	union_sm.Remove("Reset_Handler")

	/* slurp in all symbols from the actual loader binary */
	err, loader_elf_sm := t.Loader.ParseObjectElf()
	if err != nil {
		return err
	}

	/* remove the symbols from the .a files in the app files, but only if
	 * they are actually found in the elf file (not just the union) */
	util.StatusMessage(util.VERBOSITY_DEFAULT,
		"Removing Symbols from Application\n")
	for name, info1 := range *union_sm {
		if _, found := loader_elf_sm.Find(name); found {
			err := t.App.RenameSymbol(&info1, "_xxx")
			if err != nil {
				return err
			}
		}
	}

	/* copy the .elf from the loader since we want to preseve the
	 * original one for debug and download */
	/* TODO */

	/* go through each symbol in elf file and rename them if they are not in
	 * the union (meaining that we don't want to export them to the app
	 * as we could get a duplicate symbol during link). We rename them
	 * instead of deleting them as there are a few symbols that we will
	 * need in the linker (like bss ranges etc). */

	util.StatusMessage(util.VERBOSITY_DEFAULT,
		"Removing Symbols from Loader\n")
	for name, info1 := range *loader_elf_sm {
		if _, found := (*union_sm)[name]; !found {
			t.Loader.RenameSymbol(&info1, "_loader")
		}
	}

	if t.Bsp.Part2LinkerScript == "" {
		return util.NewNewtError("Must specify Part2 Linker in Bsp to support Split images")
	}

	// link the loader elf into the application. This has to be treated as
	// special (not just another object) because we have to link the whole
	// library into it
	t.App.LinkElf = t.Loader.AppElfPath()
	err = t.App.Link(t.Bsp.Part2LinkerScript)

	if err != nil {
		return err
	}

	return err
}

func (t *TargetBuilder) Test(p *pkg.LocalPackage) error {
	if err := t.target.Validate(false); err != nil {
		return err
	}

	/* TODO test the app */
	return nil
}

func (t *TargetBuilder) resolveCompiler() *pkg.LocalPackage {
	if t.Bsp.CompilerName == "" {
		return nil
	}
	dep, _ := pkg.NewDependency(t.Bsp.Repo(), t.Bsp.CompilerName)
	mypkg := project.GetProject().ResolveDependency(dep).(*pkg.LocalPackage)
	return mypkg
}

func (t *TargetBuilder) Clean() error {
	var err error

	err = t.PrepBuild()

	if err == nil && t.App != nil {
		err = t.App.Clean()
	}
	if err == nil && t.Loader != nil {
		err = t.Loader.Clean()
	}
	return err
}

func (t *TargetBuilder) GetTarget() *target.Target {
	return (*t).target
}
