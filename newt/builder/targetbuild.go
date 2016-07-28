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
	"mynewt.apache.org/newt/newt/symbol"
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
	t.compilerPkg = compilerPkg

	appPkg := t.target.App()
	targetPkg := t.target.Package()

	app, err := NewBuilder(t, "app")

	if err == nil {
		t.App = app
	} else {
		return err
	}

	loaderPkg := t.target.Loader()

	if loaderPkg != nil {
		loader, err := NewBuilder(t, "loader")

		if err == nil {
			t.Loader = loader
		} else {
			return err
		}

		err = t.Loader.PrepBuild(loaderPkg, bspPkg, targetPkg)
		if err != nil {
			return err
		}
		t.LoaderList = project.ResetDeps(nil)
	}

	bsp_pkg := t.target.Bsp()

	err = t.App.PrepBuild(appPkg, bsp_pkg, targetPkg)
	if err != nil {
		return err
	}

	t.AppList = project.ResetDeps(nil)

	return nil
}

func (t *TargetBuilder) Build() error {
	var err error
	var linkerScript string

	if err = t.target.Validate(true); err != nil {
		return err
	}

	if err = t.PrepBuild(); err != nil {
		return err
	}

	if t.Loader != nil {
		project.ResetDeps(t.LoaderList)

		if err = t.Bsp.Reload(t.Loader.Features()); err != nil {
			return err
		}

		err = t.Loader.Build()

		if err != nil {
			return err
		}

		/* build a combined app archive to link against */
		err = t.Loader.BuldAppArchive(t.Loader.AppCombinedLibPath())
		if err != nil {
			return err
		}

		/* perform the final link */
		err = t.Loader.Link(t.Bsp.LinkerScript)

		if err != nil {
			return err
		}
	}

	/* Build the Apps */
	project.ResetDeps(t.AppList)

	if err := t.Bsp.Reload(t.App.Features()); err != nil {
		return err
	}

	err = t.App.Build()

	if err != nil {
		return err
	}

	if t.Loader != nil {
		err = t.buildRomElf()
		if err != nil {
			return err
		}

		t.App.LinkElf = t.Loader.AppLinkerElfPath()
		linkerScript = t.Bsp.Part2LinkerScript
	} else {
		linkerScript = t.Bsp.LinkerScript
	}

	/* now do a Pre-link/Post archive on the application */
	err = t.App.BuldAppArchive(t.App.AppCombinedLibPath())

	if err != nil {
		return err
	}

	if linkerScript == "" {
		return util.NewNewtError("BSP Must specify Linker script ")
	}
	// link the loader elf into the application. This has to be treated as
	// special (not just another object) because we have to link the whole
	// library into it
	err = t.App.Link(linkerScript)
	if err != nil {
		return err
	}

	return err
}

func (t *TargetBuilder) buildRomElf() error {

	/* check dependencies on the ROM ELF.  This is really dependent on
	 * all of the .a files, but since we already depend on the loader
	 * .as to build the initial elf, we only need to check the app .a */
	c, err := t.NewCompiler(t.Loader.AppElfPath())
	d := toolchain.NewDepTracker(c)
	if err != nil {
		return err
	}

	archNames := []string{}

	/* build the set of archive file names */
	for _, bpkg := range t.App.Packages {
		archivePath := t.App.ArchivePath(bpkg.Name())
		if util.NodeExist(archivePath) {
			archNames = append(archNames, archivePath)
		}
	}

	bld, err := d.RomElfBuldRequired(t.Loader.AppLinkerElfPath(), t.Loader.AppElfPath(), archNames)
	if err != nil {
		return err
	}

	if !bld {
		return nil
	}

	util.StatusMessage(util.VERBOSITY_DEFAULT,
		"Generating ROM elf \n")

	err, app_sm := t.App.FetchSymbolMap()
	if err != nil {
		return err
	}

	err, loader_sm := t.Loader.FetchSymbolMap()
	if err != nil {
		return err
	}

	union_sm := symbol.IdenticalUnion(loader_sm, app_sm, true)

	/* handle special symbols */
	union_sm.Remove("Reset_Handler")

	/* slurp in all symbols from the actual loader binary */
	err, loader_elf_sm := t.Loader.ParseObjectElf()
	if err != nil {
		return err
	}

	final_sm := symbol.IdenticalUnion(union_sm, loader_elf_sm, false)

	/* NOTE: there is one special symbol we need in this image which
	 * tells the split image linker how much RAM it is using HeapBase. We also
	 * want to name it something else */
	heapBaseSymbol := symbol.NewSymbolInfo()
	heapBaseSymbol.Name = "__HeapBase"
	heapBaseSymbol.Ext = ".elf"
	final_sm.Add(*heapBaseSymbol)

	err = t.Loader.CopySymbols(final_sm)
	if err != nil {
		return err
	}

	err = t.Loader.RenameSymbol(heapBaseSymbol, "_loader")

	if err != nil {
		return err
	}

	return nil
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
