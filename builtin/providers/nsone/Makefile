.PHONY: all clean test

all: terraform-provider-nsone .git/hooks/pre-commit

install: terraform-provider-nsone
	cp -f terraform-provider-nsone $$(dirname $$(which terraform))

terraform-provider-nsone: main.go nsone/*.go
	go build .

fmt:
	go fmt ./...

test: .git/hooks/pre-commit
	cd nsone ; go test -v .

clean:
	rm -f terraform-provider-nsone
	make -C yelppack clean

.git/hooks/pre-commit:
	    if [ ! -f .git/hooks/pre-commit ]; then ln -s ../../git-hooks/pre-commit .git/hooks/pre-commit; fi

itest_%:
	mkdir -p dist
	make -C yelppack $@

package: itest_trusty

