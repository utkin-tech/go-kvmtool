package demo

import "os"

type Sockets struct {
	ParentToChildReader *os.File
	ParentToChildWriter *os.File
	ChildToParentReader *os.File
	ChildToParentWriter *os.File
}

func (s *Sockets) Close() []error {
	errors := []error{}

	if err := s.ParentToChildReader.Close(); err != nil {
		errors = append(errors, err)
	}

	if err := s.ParentToChildWriter.Close(); err != nil {
		errors = append(errors, err)
	}

	if err := s.ChildToParentReader.Close(); err != nil {
		errors = append(errors, err)
	}

	if err := s.ChildToParentWriter.Close(); err != nil {
		errors = append(errors, err)
	}

	return errors
}

func CreateSockets() (*Sockets, error) {
	parentToChildReader, parentToChildWriter, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	childToParentReader, childToParentWriter, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	return &Sockets{
		ParentToChildReader: parentToChildReader,
		ParentToChildWriter: parentToChildWriter,
		ChildToParentReader: childToParentReader,
		ChildToParentWriter: childToParentWriter,
	}, err
}
