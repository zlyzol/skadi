package common

func Try(cnt int, waitMs int, f func() error) (err error) {
	for i := 0; i < cnt; i++ {
		err = f()
		if err == nil { return nil }
	}
	return err
}
