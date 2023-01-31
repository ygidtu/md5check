# md5check

Multithreading md5sum and check tool

```bash
Usage: md5check [global options] 

Global options:
        -i, --input   The path to file or directory (default: ./)
        -o, --output  The path to output file, default save to stdout
        -t, --thread  How many threads to use (default: 4)
        -c, --check   Check exist md5
        -r, --resume  Resume and skip finished files
        -f, --hidden  Calculate md5 to hidden files
            --debug   Show debug information
        -v, --version Show version information
        -h, --help    Show this help
```

Similar to md5sum from linux with multithreading and resumed from last unfished progress

```bash
# generate md5 and print to console
md5check -i ./ -t 10

# generate md5 and save to given file
md5check -i ./ -o md5sum.txt -t 10

# check the existed md5 and print to console
md5check -c md5sum.txt

# check the existed md5 and save to given file, but support relative path
md5check -c ../example/md5sum.txt -i ../example -o md5check.txt
```
