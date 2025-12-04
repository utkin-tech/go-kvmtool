```
# create the top most bundle directory
mkdir tmp
cd tmp

# create the rootfs directory
mkdir rootfs

# export busybox via Docker into the rootfs directory
docker export $(docker create alpine) | tar -C rootfs -xvf -

runc spec
```

Start gkvm

```
sudo ./bin/gkvm create --bundle $(pwd)/tmp 12
```

Start using docker

```
docker run -it --rm --runtime gkvm-shim alpine:latest
```

View containerd tasks

```
/run/containerd/io.containerd.runtime.v2.task/moby/
```

View containerd logs

```
journalctl -u containerd
```
