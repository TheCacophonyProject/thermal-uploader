# thermal-uploader

This software is used by The Cacophony Project to upload thermal video
recordings in CPTV format to the project's API server. These
recordings are typically created by the
[thermal-recorder](https://github.com/TheCacophonyProject/thermal-recorder/).

## Releases

Releases are built using TravisCI. To create a release:

* Tag the release with an annotated tag. For example:
  `git tag -a "v1.4" -m "1.4 release"`
* Push the tag to Github: `git push origin v1.4`
* TravisCI will see the pushed tag, run the tests, create a release
  package and create a
  [Github Release](https://github.com/TheCacophonyProject/thermal-uploader/releases).

For more about the mechanics of how releases work, see `travis.yml`
and `.goreleaser.yml`.
