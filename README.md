# gce-csi-driver
Working directory for GCE CSI driver

## Testing

### Testing CreateVolume
csc controller create-volume csi-test --req-bytes=5000000000 --lim-bytes=5000000000 --params type=pd-standard,zone=us-central1-b --cap SINGLE_NODE_WRITER,mount,fs -v 0.1.0 --endpoint="/tmp/csi.sock"

### Testing DeleteVolume
csc controller delete-volume dyzz-test/us-central1-b/csi-test -v 0.1.0 --endpoint="/tmp/csi.sock"

### Testing ControllerPublishVolume
csc controller publish --node-id="dyzz-test/us-central1-b/csi-test-instance" "dyzz-test/us-central1-b/csi-test" --cap=SINGLE_NODE_WRITER,mount,fs --endpoint="/tmp/csi.sock""dyzz-test/us-central1-b/csi-test"

### Testing ControllerUnpublishVolume
csc controller unpublish --node-id="dyzz-test/us-central1-b/csi-test-instance" "dyzz-test/us-central1-b/csi-test" --endpoint="/tmp/csi.sock"
