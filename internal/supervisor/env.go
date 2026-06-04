package supervisor

import "os"

func envFromParent() []string {
	return os.Environ()
}
