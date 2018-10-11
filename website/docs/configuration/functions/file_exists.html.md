---
layout: "functions"
page_title: "file_exists function"
sidebar_current: "docs-funcs-file-file-exists"
description: |-
  The file_exists function determines whether a file exists at a given path.
---

# `file_exists` Function

`file_exists` determines whether a file exists at a given path

```hcl
file_exists(path)
```

## Examples

```
> file_exists("${path.module}/hello.txt")
true
```

```hcl

file_exists("custom-section.sh") ? file("custom-section.sh") : local.default_content
```

## Related Functions

* [`file`](./file.html) reads the contents of a file at a given path
