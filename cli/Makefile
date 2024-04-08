.PHONY: default

EXE=

ifeq ($(OS),Windows_NT)
EXE=.exe
endif


NAME=lamacli

default: $(NAME)

.PHONY: $(NAME)
$(NAME):
	go build 

clean:
	rm -f $(NAME)$(EXE)
