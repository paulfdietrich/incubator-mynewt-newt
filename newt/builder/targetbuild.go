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

		err = t.Loader.Link()

		if err != nil {
			return err
		}

		err, loader_sm = t.Loader.FetchSymbolMap()
		if err != nil {
			return err
		}

		loader_sm.Dump()

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

	err, app_sm := t.App.FetchSymbolMap()
	if err != nil {
		return err
	}

	app_sm.Dump()

	fmt.Printf("Combining maps into the overlap\n ")

	union_sm := IdenticalUnion(loader_sm, app_sm)

	union_sm.Dump()

	/* remove the symbols from the .a files in the app files */

	for name, info1 := range *union_sm {
		fmt.Printf("Removing symbol %s from %s Library\n", name, info1.bpkg)
		err := t.App.RemoveSymbol(&info1)
		if err != nil {
			fmt.Printf("Error Removing symbol %s from app Library\n", name, err.Error())
		}
	}

	/* copy the .elf from the loader */
	/* slurp in all symbols */
	/* go through each symbol in elf copy */
	/* drop if not in union */

	/* go through each symbol in the union... */
	/* drop from apps libraries */

	err = t.App.Link()

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
