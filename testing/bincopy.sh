#!/bin/bash

scp -i ~/.ssh/google_compute_engine_encrypted ./bin/{gce-csi-driver,csc} dyzz@35.188.104.200:~/
