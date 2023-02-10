SOURCEDIR  := info/kwarc/kalah/
JAVAFILES  != find $(SOURCEDIR) -name '*.java'
JCOPTS     := -cp $(SOURCEDIR) -g -Xlint:all

.PHONY: all
all: jkgp.jar

jkgp.jar: $(JAVAFILES)
	javac $(JCOPTS) $(JAVAFILES)
	jar cf $@ info/
