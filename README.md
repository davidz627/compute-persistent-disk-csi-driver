# gce-csi-driver
Working directory for GCE CSI driver

## Testing

### Testing CreateVolume
csc controller create-volume csi-test --req-bytes=5000000000 --lim-bytes=5000000000 --params type=pd-standard,zone=us-central1-b --cap SINGLE_NODE_WRITER,mount,fs --endpoint="/tmp/csi.sock" -v 0.2.0

### Testing DeleteVolume
csc controller delete-volume dyzz-test/us-central1-b/csi-test --endpoint="/tmp/csi.sock" -v 0.2.0

### Testing ControllerPublishVolume
csc controller publish --node-id="dyzz-test/us-central1-b/csi-test-instance" "dyzz-test/us-central1-b/csi-test" --cap=SINGLE_NODE_WRITER,mount,fs --endpoint="/tmp/csi.sock" -v 0.2.0

### Testing ControllerUnpublishVolume
csc controller unpublish --node-id="dyzz-test/us-central1-b/csi-test-instance" "dyzz-test/us-central1-b/csi-test" --endpoint="/tmp/csi.sock" -v 0.2.0

### Testing NodePublishVolume
csc node publish "dyzz-test/us-central1-b/csi-test" --cap=SINGLE_NODE_WRITER,mount,fs --target-path="/usr/local/google/home/dyzz/test/test-mount" --endpoint="/tmp/csi.sock" -v 0.2.0 

### Testing NodeUnpublishVolume
csc node unpublish "dyzz-test/us-central1-b/csi-test" --target-path="test-mount" --endpoint="/tmp/csi.sock" -v 0.2.0