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
	"fmt"

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

		loader_flag := toolchain.NewCompilerInfo()
		loader_flag.Cflags = append(loader_flag.Cflags, "-DSPLIT_LOADER")
		t.Loader.AddCompilerInfo(loader_flag)

		t.LoaderList = project.ResetDeps(nil)
	}

	bsp_pkg := t.target.Bsp()

	err = t.App.PrepBuild(appPkg, bsp_pkg, targetPkg)
	if err != nil {
		return err

	}
	if loaderPkg != nil {
		app_flag := toolchain.NewCompilerInfo()
		app_flag.Cflags = append(app_flag.Cflags, "-DSPLIT_APPLICATION")
		t.App.AddCompilerInfo(app_flag)
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

	/* Build the Apps */
	project.ResetDeps(t.AppList)

	if err := t.Bsp.Reload(t.App.Features(t.App.BspPkg)); err != nil {
		return err
	}

	err = t.App.Build()
	if err != nil {
		return err
	}

	/* if we have no loader, we are done here.  All of the rest of this
	 * function is for split images */
	if t.Loader == nil {
		err = t.App.Link(t.Bsp.LinkerScript)
		return err
	}

	err = t.App.TestLink(t.Bsp.LinkerScript)
	if err != nil {
		return err
	}

	/* fetch symbols from the elf and from the libraries themselves */
	err, appLibSym := t.App.ExtractSymbolInfo()
	if err != nil {
		return err
	}

	err, appElfSym := t.App.ParseObjectElf(t.App.AppTempElfPath())
	if err != nil {
		return err
	}

	project.ResetDeps(t.LoaderList)

	if err = t.Bsp.Reload(t.Loader.Features(t.Loader.BspPkg)); err != nil {
		return err
	}

	err = t.Loader.Build()

	if err != nil {
		return err
	}

	/* perform the final link */
	err = t.Loader.TestLink(t.Bsp.LinkerScript)

	if err != nil {
		return err
	}

	err, loaderLibSym := t.Loader.ExtractSymbolInfo()
	if err != nil {
		return err
	}

	err, loaderElfSym := t.Loader.ParseObjectElf(t.Loader.AppTempElfPath())
	if err != nil {
		return err
	}

	err, sm_match, sm_nomatch := symbol.IdenticalUnion(appLibSym, loaderLibSym, true, false)
	fmt.Println(len(*sm_match), " symbols matched in library files ")

	/* which packages are shared between the two */
	common_pkgs := sm_match.Packages()
	uncommon_pkgs := sm_nomatch.Packages()

	for v, _ := range uncommon_pkgs {
		if t.App.appPkg != nil && t.App.appPkg.Name() != v &&
			t.Loader.appPkg != nil && t.Loader.appPkg.Name() != v {
			trouble := sm_nomatch.FilterPkg(v)

			header := true
			for _, sym := range *trouble {
				if !sym.IsLocal() {
					if header {
						fmt.Println("We have non-matching global symbols in ", v)
						header = false
					}
					sym.Dump()
				}
			}

			if !header {
				fmt.Println("We cannot combine package ", v, " between app and loader ")
				delete(common_pkgs, v)
				return util.NewNewtError("Common package has different implementaiton")
			}
		}
	}

	/* The app can ignore these packages next time */
	t.App.RemovePackages(common_pkgs)

	/* add back the BSP package which needs linking in both */
	t.App.AddPackage(t.Bsp.LocalPackage)

	/* for each symbol in the elf of the app, if that symbol is in
	 * a common package, keep that symbol in the loader */
	preserve_elf := symbol.NewSymbolMap()

	/* go through each symbol in the app */
	for _, elfsym := range *appElfSym {
		name := elfsym.Name
		if libsym, ok := (*appLibSym)[name]; ok {
			if _, ok := common_pkgs[libsym.Bpkg]; ok {
				/* if its not in the loader elf, add it as undefined */
				if _, ok := (*loaderElfSym)[name]; !ok {
					preserve_elf.Add(elfsym)
				}
			}
		}
	}

	/* re-link loader */
	project.ResetDeps(t.LoaderList)

	/* perform the final link of the loader */
	fmt.Println("Migrating ", len(*preserve_elf), " symbols to Loader")
	preserve_elf.Dump("Preserving")
	err = t.Loader.KeepLink(t.Bsp.LinkerScript, preserve_elf)

	if err != nil {
		return err
	}

	/* create the special elf to link the app against */
	/* its just the elf with a set of symbols removed and renamed */
	err = t.buildRomElf()
	if err != nil {
		return err
	}

	t.App.LinkElf = t.Loader.AppLinkerElfPath()

	linkerScript = t.Bsp.Part2LinkerScript

	if linkerScript == "" {
		return util.NewNewtError("BSP Must specify Linker script ")
	}

	t.App.LinkElf = t.Loader.AppLinkerElfPath()
	err = t.App.Link(linkerScript)

	if err != nil {
		return err
	}

	/* some debug to dump out the interesting stuff from the elfs */
	err, final_ap_sm := t.App.ParseObjectLibraryFile(nil, t.App.AppElfPath(), false)

	if err != nil {
		return err
	}

	err, final_loader_sm := t.Loader.ParseObjectLibraryFile(nil, t.Loader.AppElfPath(), false)

	if err != nil {
		return err
	}

	err, sm_match, sm_nomatch = symbol.IdenticalUnion(final_ap_sm, final_loader_sm, true, true)

	sm_nomatch.GlobalDataOnly().Dump("non matching Global Data symbols")
	sm_nomatch.GlobalFunctionsOnly().Dump("non matching Global Code symbols")

	return nil
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
	for _, bpkg := range t.Loader.Packages {
		archivePath := t.Loader.ArchivePath(bpkg.Name())
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

	/* slurp in all symbols from the actual loader binary */
	err, loader_elf_sm := t.Loader.ParseObjectElf(t.Loader.AppElfPath())
	if err != nil {
		return err
	}

	/* handle special symbols */

	/* Make sure this is not shared as this is what links in the
	 * entire application (essential the root of the function tree */
	loader_elf_sm.Remove("main")
	loader_elf_sm.Remove("_start")
	loader_elf_sm.Remove("__StackTop")
	loader_elf_sm.Remove("__HeapLimit")
	loader_elf_sm.Remove("__StackLimit")

	err = t.Loader.CopySymbols(loader_elf_sm)
	if err != nil {
		return err
	}

	/* These symbols are needed by the split app so it can zero
	 * bss and copy data from the loader app before it restarts,
	 * but we have to rename them since it has its own copies of
	 * these special linker symbols  */
	tmp_sm := symbol.NewSymbolMap()
	tmp_sm.Add(*symbol.NewElfSymbol("__HeapBase"))
	tmp_sm.Add(*symbol.NewElfSymbol("__bss_start__"))
	tmp_sm.Add(*symbol.NewElfSymbol("__bss_end__"))
	tmp_sm.Add(*symbol.NewElfSymbol("__etext"))
	tmp_sm.Add(*symbol.NewElfSymbol("__data_start__"))
	tmp_sm.Add(*symbol.NewElfSymbol("__data_end__"))
	err = c.RenameSymbols(tmp_sm, t.Loader.AppLinkerElfPath(), "_loader")

	if err != nil {
		return err
	}
	return nil
}

func (t *TargetBuilder) Test(p *pkg.LocalPackage) error {
	if err := t.target.Validate(false); err != nil {
		return err
	}

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

	targetPkg := t.target.Package()

	app, err := NewBuilder(t, "test")

	if err == nil {
		t.App = app
	} else {
		return err
	}

	// A few features are automatically supported when the test command is
	// used:
	//     * TEST:      ensures that the test code gets compiled.
	//     * SELFTEST:  indicates that there is no app.
	t.App.AddFeature("TEST")
	t.App.AddFeature("SELFTEST")

	err = t.App.PrepBuild(p, bspPkg, targetPkg)

	if err != nil {
		return err
	}

	err = t.App.Test(p)

	return err
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
