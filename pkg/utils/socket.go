package utils

import "path"

const SocketFile = "gkvm.sock"

func SocketDir(root string, containerId string) string {
	return path.Join(root, containerId)
}

func SocketPath(root string, containerId string) string {
	return path.Join(root, containerId, SocketFile)
}
