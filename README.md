# btlr
*Warning*: This project is experimental, and is not an officially supported 
Google product.

`btlr` is a CLI for making it easier to run commands consistently. 

## Usage

### Run

`btlr run PATTERN -- SUBCOMMAND` runs a specific command in parallel, targeting 
multiple directories concurrently. `PATTERN` is a pattern to match files 
against that supports bash-style expansion. Any directory containing a file that
matches the provided pattern will have `SUBCOMMAND` executed inside of it.

```bash
$ ./btlr run "**/*.txt" -- echo I'm in folder \"$(pwd)\"!
Running command... [2 of 2 complete].

#
# path/to/folder1
#

I'm in folder "path/to/folder1"!


#
# path/to/folder2
#

I'm in folder "path/to/folder1"!


#
# Summary 
#

path/to/folder1.......................................................[SUCCESS]
path/to/folder2.......................................................[SUCCESS]
```
