```
# create the top most bundle directory
mkdir mycontainer
cd mycontainer

# create the rootfs directory
mkdir rootfs

# export busybox via Docker into the rootfs directory
docker export $(docker create busybox) | tar -C rootfs -xvf -

runc spec

runc create 321

ps -eo pid,ppid,user,comm
```