package jcache

import "io/ioutil"

func redirectArgOption(argsIn []string, option, value string, addIfNotExists bool) []string {
	// re-pack compiler arguments
	optIdx := optionIndexOf(argsIn, option)
	if optIdx >= 0 {
		args := make([]string, len(argsIn))
		copy(args, argsIn)

		args[optIdx+1] = value
		return args
	}
	if !addIfNotExists {
		return argsIn
	}

	// we'll need to add our classes-out-dir
	args := make([]string, len(argsIn)+2)
	args[0] = option
	args[1] = value
	for i := 0; i < len(argsIn); i++ {
		args[i+2] = argsIn[i]
	}
	return args
}
func optionIndexOf(args []string, option string) int {
	for i, arg := range args {
		if arg == option {
			return i
		}
	}

	return -1
}
func writeArgsToTmpFile(args []string) (filename string, err error) {
	file, err := ioutil.TempFile("", "jcache_args")
	if err != nil {
		return
	}
	defer file.Close()

	filename = file.Name()

	for _, arg := range args {
		//file.WriteString("\"" + arg + "\"\n")
		file.WriteString(arg + "\n")
	}

	return
}
