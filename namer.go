package dagger

import (
	"fmt"
	"reflect"
	"regexp"
	"runtime"
	"strings"
)

// StepNamer is the authoritative provider of a step's name.
// It can be used to inspect a step's name even if the step is wrapped
// with a middleware step.
type StepNamer interface {
	StepName() fmt.Stringer
}

// ScopedName holds the package name and function name of the StepFunc.
type ScopedName [2]string

// GenericScopedName holds the info about Step[S] and S.
type GenericScopedName [2]ScopedName

func (s ScopedName) Module() string {
	pp := s.PackagePath()
	lastIndex := strings.LastIndex(pp, "/")
	if lastIndex == -1 {
		return ""
	}
	return pp[:lastIndex]
}
func (s ScopedName) Package() string {
	pp := s.PackagePath()
	lastIndex := strings.LastIndex(pp, "/")
	if lastIndex == -1 {
		return pp
	}
	return pp[lastIndex+1:]
}
func (s ScopedName) PackagePath() string { return s[0] }
func (s ScopedName) Name() string        { return s[1] }
func (s ScopedName) String() string {
	pkg := s.Package()
	if pkg == "" {
		return s.Name()
	}

	return strings.Join([]string{
		pkg,
		s.Name(),
	}, ":")
}

func (s GenericScopedName) StepScopedName() ScopedName { return s[0] }
func (s GenericScopedName) TypeScopedName() ScopedName { return s[1] }
func (s GenericScopedName) String() string {
	return fmt.Sprintf("%s[%s]", s[0].String(), s[1].Name())
}

// StepName returns the name of a step.
//
// Any arbitrary Step can also implement
// `StepName() string` to use a custom way
// to naming the Step.
func StepName[S any](s Step[S]) fmt.Stringer {
	switch s := s.(type) {
	case StepNamer:
		return s.StepName()
	case interface{ StepName() string }:
		return fmtStr(s.StepName())
	case StepFunc[S]:
		pkgName, fnName := stepFuncName(s)

		return ScopedName{pkgName, fnName}
	}

	return stepTypeName(s)
}

// private API

const (
	moduleNamedGroup  = "module"
	pkgNamedGroup     = "pkg"
	typeNamedGroup    = "type"
	structNamedGroup  = "struct"
	pointerNamedGroup = "pointer"
	methodNamedGroup  = "method"
)

var (
	// This regex parses the <module?>/<package?>.<type> like strings
	// to extract important information about Steps.
	// By default, reflect package spits out long names,
	// which can bloat the debugging experience.
	//
	// Tip: You may get a concrete value of any regex
	// by calling `.String()` on it, and then use
	// an explainer online like https://regex101.com
	// to understand the whole thing.
	typeNameRegex = fmt.Sprintf(
		`(((?P<%s>.+?)\/)?(?P<%s>[^\/.]+)\.)?(?P<%s>.+)`,
		moduleNamedGroup,
		pkgNamedGroup,
		typeNamedGroup,
	)

	runtimeStepNameExtractor        = regexp.MustCompile(fmt.Sprintf(`^%s$`, typeNameRegex))
	runtimeGenericTypeNameExtractor = regexp.MustCompile(fmt.Sprintf(
		`^(?P<%s>\w+)\[%s]$`,
		structNamedGroup,
		typeNameRegex,
	))
	structMethodExtractor = regexp.MustCompile(fmt.Sprintf(
		`^((\(\*(?P<%s>\w+)\))|(?P<%s>\w+))\.(?P<%s>\w+)-fm$`,
		pointerNamedGroup,
		structNamedGroup,
		methodNamedGroup,
	))

	stepModuleIndex = runtimeStepNameExtractor.SubexpIndex(moduleNamedGroup)
	stepPkgIndex    = runtimeStepNameExtractor.SubexpIndex(pkgNamedGroup)
	stepFnIndex     = runtimeStepNameExtractor.SubexpIndex(typeNamedGroup)

	structIndex        = runtimeGenericTypeNameExtractor.SubexpIndex(structNamedGroup)
	genericModuleIndex = runtimeGenericTypeNameExtractor.SubexpIndex(moduleNamedGroup)
	genericPkgIndex    = runtimeGenericTypeNameExtractor.SubexpIndex(pkgNamedGroup)
	genericTypeIndex   = runtimeGenericTypeNameExtractor.SubexpIndex(typeNamedGroup)

	pointerIndex    = structMethodExtractor.SubexpIndex(pointerNamedGroup)
	structNameIndex = structMethodExtractor.SubexpIndex(structNamedGroup)
	methodNameIndex = structMethodExtractor.SubexpIndex(methodNamedGroup)
)

type fmtStr string

func (f fmtStr) String() string { return string(f) }

func stepFuncName[S any](s Step[S]) (string, string) {
	pkgPath := "UnknownPackagePath"
	fnName := "UnknownFunc"

	if fnPtr := runtime.FuncForPC(reflect.ValueOf(s).Pointer()); fnPtr != nil {
		fullName := fnPtr.Name()

		if matches := runtimeStepNameExtractor.FindStringSubmatch(fullName); len(matches) > 0 {
			pkgPath = fmtPkgPath(matches[stepModuleIndex], matches[stepPkgIndex])
			fnName = matches[stepFnIndex]
		}
	}

	if matches := structMethodExtractor.FindStringSubmatch(fnName); len(matches) > 0 {
		smName := fmt.Sprintf("%s.%s", matches[structNameIndex], matches[methodNameIndex])
		if matches[pointerIndex] != "" {
			smName = fmt.Sprintf("*%s.%s", matches[pointerIndex], matches[methodNameIndex])
		}
		fnName = smName
	}

	return pkgPath, fnName
}

func stepTypeName[S any](s Step[S]) fmt.Stringer {
	t := reflect.TypeOf(s)

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if matches := runtimeGenericTypeNameExtractor.FindStringSubmatch(t.Name()); len(matches) > 0 {
		isPtr := false
		genModule := matches[genericModuleIndex]
		if strings.HasPrefix(genModule, "*") {
			genModule = genModule[1:]
			isPtr = true
		}

		genPkg := matches[genericPkgIndex]
		if strings.HasPrefix(genPkg, "*") {
			genPkg = genPkg[1:]
			isPtr = true
		}

		genType := matches[genericTypeIndex]
		if isPtr {
			genType = "*" + genType
		}

		return GenericScopedName{
			ScopedName{t.PkgPath(), matches[structIndex]}, // ScopedName for Step[S]
			ScopedName{
				fmtPkgPath(genModule, genPkg),
				genType,
			}, // ScopedName for S
		}
	}

	return ScopedName{t.PkgPath(), t.Name()}
}

func fmtPkgPath(mod, pkg string) string {
	if mod == "" && pkg == "" {
		return ""
	}

	if mod == "" {
		return pkg
	}

	return fmt.Sprintf("%s/%s", mod, pkg)
}
