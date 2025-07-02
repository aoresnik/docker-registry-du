# docker-registry-du
Report storage usage of a Docker Registry. Reports usage by image and tag, shared and exclusive usage.

WARNING: This software is in prototype phase - do not use on production repositories.

## Installing

1. You need a working Go installation
1. Checkout project
1. Run build
    ```
    go build
    ```
1. Run the code
    ```
    ./docker-registry-du -username user -password password https://your-registry.example.com
    ```
    