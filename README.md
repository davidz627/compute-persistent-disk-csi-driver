# gce-csi-driver
Working directory for GCE CSI driver

## Testing

###Testing CreateVolume
csc controller create-volume csi-test --req-bytes=5000000000 --lim-bytes=5000000000 --params type=pd-standard,zone=us-central1-b -v 0.1.0 --endpoint="/tmp/csi.sock"