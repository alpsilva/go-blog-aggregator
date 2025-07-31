package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/alpsilva/config"
	"github.com/alpsilva/go-blog-aggregator.git/internal/database"
	"github.com/google/uuid"

	_ "github.com/lib/pq"
)

type state struct {
	db  *database.Queries
	cfg *config.Config
}

type command struct {
	name string
	args []string
}

type commands struct {
	availableCommands map[string]func(*state, command) error
}

func (c commands) run(s *state, cmd command) error {
	handler, ok := c.availableCommands[cmd.name]
	if !ok {
		return errors.New("command not found")
	}

	err := handler(s, cmd)
	if err != nil {
		return err
	}

	return nil
}

func (c commands) register(name string, f func(*state, command) error) {
	c.availableCommands[name] = f
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return errors.New("no username provided")
	}

	userName := cmd.args[0]

	user, err := s.db.GetUser(context.Background(), userName)
	if err != nil {
		return errors.New("user does not exist")
	}

	err = s.cfg.SetUser(user.Name)
	if err != nil {
		return err
	}

	fmt.Println("User has been set to:", s.cfg.CurrentUserName)

	return nil
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return errors.New("no username provided")
	}

	_, err := s.db.GetUser(context.Background(), cmd.args[0])
	if err == nil {
		return errors.New("user already exists")
	}

	params := database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      cmd.args[0],
	}

	newUser, err := s.db.CreateUser(context.Background(), params)
	if err != nil {
		return err
	}

	s.cfg.SetUser(newUser.Name)

	fmt.Printf("User %s has been created with id %s\n", newUser.Name, newUser.ID.String())

	return nil
}

func handlerReset(s *state, cmd command) error {

	err := s.db.ResetUsers(context.Background())
	if err != nil {
		fmt.Println("Reset unsuccessful")
		os.Exit(1)
	}

	fmt.Println("Reset Successful")

	return nil
}

func main() {

	var err error
	cfg := config.Read()

	db, err := sql.Open("postgres", cfg.DbUrl)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	dbQueries := database.New(db)

	stateStc := state{
		db:  dbQueries,
		cfg: &cfg,
	}

	commandsStc := commands{
		availableCommands: make(map[string]func(*state, command) error),
	}

	commandsStc.register("login", handlerLogin)
	commandsStc.register("register", handlerRegister)
	commandsStc.register("reset", handlerReset)

	args := os.Args

	if len(args) < 2 {
		fmt.Println("Not enough arguments provided")
		os.Exit(1)
	}

	cmdName := args[1]
	cmdArgs := args[2:]

	cmd := command{
		name: cmdName,
		args: cmdArgs,
	}

	err = commandsStc.run(&stateStc, cmd)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
