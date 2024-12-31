# jira-to-markdown

## Step 1

### Performance considerations

The program creates millions of small files. To speed this up / save write cycles on your SSD, you can set the working directory to a ramdisk before running.

* MacOS: https://gist.github.com/rxin/5085564
  * APFS might perform better than HFS+: https://superuser.com/a/1772162/147658
  * Mounted drive appears at `/Volumes/ramdisk`
  * The entities.xml I tested with is ~960 MB. ~3.5 million temp files are created. When using APFS and `RAMDISK_SIZE_MB=17000`, the temp files fit without much space to spare.

The 'remainder' file is append-only and relatively few large writes, you may put this
anywhere to save space on your ramdisk.

### Handling 'XML syntax error ... illegal character code'

```
XML syntax error on line 4805102: illegal character code U+001D
```

The U+001D is ASCII `GS` (Group Separator). Shown in `less` as `^]`. It was included in a CDATA section.

In XML 1.0 this is disallowed and in 1.1 it is "only valid in certain contexts ... restricted and highly discouraged" according to [Wikipedia 'Valid characters in XML'](https://en.wikipedia.org/wiki/Valid_characters_in_XML).

```zsh
% less entities.xml
# `4805102g` to jump to the line from the error
# In context the ASCII GS may be quite intentional so we will keep it, replace it with "{GS}"

% grep -bo "\x1D" jira-export/entities.xml | wc -l
       1
% grep -bo "{GS}" jira-export/entities.xml | wc -l
       0

# Below, $'\x1D' lets the shell resolve the character reference then pass through to sed.

# Mac uses BSD sed which is different: https://stackoverflow.com/q/4247068
# install 'gsed':
% brew install gnu-sed

% gsed -i.bak s/$'\x1D'/{GS}/ jira-export/entities.xml

% diff jira-export/entities.xml jira-export/entities.xml.bak
# only the 1 expected diff
 ```