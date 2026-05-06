package camera

type Camera interface {
	Capture() ([]byte, error)
}
