package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/alpsilva/config"
	"github.com/alpsilva/go-blog-aggregator.git/internal/database"
	"github.com/google/uuid"

	"github.com/lib/pq"
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

type RSSFeed struct {
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
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

	fmt.Println("User has been set to:", user.Name)

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

	err = s.cfg.SetUser(newUser.Name)
	if err != nil {
		return err
	}

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

func handlerListUsers(s *state, cmd command) error {

	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		return err
	}

	activeUserName := s.cfg.CurrentUserName

	for _, user := range users {
		output := "* " + user.Name

		if user.Name == activeUserName {
			output += " (current)"
		}

		fmt.Println(output)
	}

	return nil
}

func handlerAddFeed(s *state, cmd command, user database.User) error {

	if len(cmd.args) < 2 {
		return errors.New("not enough arguments. needs name and url")
	}

	params := database.CreateFeedParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:    user.ID,
		Name:      cmd.args[0],
		Url:       cmd.args[1],
	}

	newFeed, err := s.db.CreateFeed(context.Background(), params)
	if err != nil {
		return err
	}

	followParams := database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:    user.ID,
		FeedID:    newFeed.ID,
	}

	_, err = s.db.CreateFeedFollow(context.Background(), followParams)
	if err != nil {
		return err
	}

	fmt.Println(newFeed)

	return nil
}

func handlerListFeeds(s *state, cmd command) error {

	feeds, err := s.db.GetFeeds(context.Background())
	if err != nil {
		return err
	}

	for _, feed := range feeds {
		user, err := s.db.GetUserById(context.Background(), feed.UserID)
		if err != nil {
			return err
		}

		fmt.Printf("* %s - %s (%s)\n", feed.Name, feed.Url, user.Name)
	}

	return nil
}

func handlerFollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 1 {
		return errors.New("not enough arguments. needs url")
	}

	feed, err := s.db.GetFeedByUrl(context.Background(), cmd.args[0])
	if err != nil {
		return err
	}

	params := database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:    user.ID,
		FeedID:    feed.ID,
	}

	followRecord, err := s.db.CreateFeedFollow(context.Background(), params)
	if err != nil {
		return err
	}

	fmt.Println(followRecord)

	return nil
}

func handlerUnfollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 1 {
		return errors.New("not enough arguments. needs url")
	}

	feed, err := s.db.GetFeedByUrl(context.Background(), cmd.args[0])
	if err != nil {
		return err
	}

	params := database.DeleteFeedFollowParams{
		UserID: user.ID,
		FeedID: feed.ID,
	}

	err = s.db.DeleteFeedFollow(context.Background(), params)
	if err != nil {
		return err
	}

	return nil
}

func handlerListFollows(s *state, cmd command, user database.User) error {
	feeds, err := s.db.GetFeedFollowsForUser(context.Background(), user.ID)
	if err != nil {
		return err
	}

	output := ""
	for i, feed := range feeds {
		output += fmt.Sprintf("%d - %s\n", i+1, feed.Name)
	}

	fmt.Println(output)

	return nil
}

func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {

	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		fmt.Println(err)
		return &RSSFeed{}, err
	}
	req.Header.Add("User-Agent", "gator")

	client := http.Client{}
	response, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return &RSSFeed{}, err
	}
	body := response.Body
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		fmt.Println(err)
		return &RSSFeed{}, err
	}

	rssFeed := RSSFeed{}
	err = xml.Unmarshal(data, &rssFeed)
	if err != nil {
		fmt.Println(err)
		return &RSSFeed{}, err
	}

	return &rssFeed, nil
}

func scrapeFeeds(s *state) error {
	nextFeed, err := s.db.GetNextFeedToFetch(context.Background())
	if err != nil {
		return err
	}

	err = s.db.MarkFeedFetched(context.Background(), nextFeed.ID)
	if err != nil {
		return err
	}

	feed, err := fetchFeed(context.Background(), nextFeed.Url)
	if err != nil {
		return err
	}

	feed.Channel.Title = html.UnescapeString(feed.Channel.Title)
	feed.Channel.Description = html.UnescapeString(feed.Channel.Description)
	for _, rssItem := range feed.Channel.Item {
		rssItem.Title = html.UnescapeString(rssItem.Title)
		rssItem.Description = html.UnescapeString(rssItem.Description)
	}

	for _, item := range feed.Channel.Item {
		publishedAt, err := time.Parse("02/01/2006 15:04:05", item.PubDate)
		if err != nil {
			return fmt.Errorf("error parsing date: %s", item.PubDate)
		}

		params := database.CreatePostParams{
			ID:          uuid.New(),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Title:       item.Title,
			Url:         item.Link,
			Description: sql.NullString{String: item.Description, Valid: true},
			PublishedAt: publishedAt,
			FeedID:      nextFeed.ID,
		}

		_, err = s.db.CreatePost(context.Background(), params)
		if err != nil {
			var pqErr *pq.Error
			if errors.Is(err, pqErr) && pqErr.Code == "23505" {
				fmt.Printf("post with url '%s' duplicated. Ignoring...", params.Url)
			} else {
				return err
			}
		}
	}

	return nil
}

func handlerAgg(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return errors.New("missing arg 'time_between_reqs'")
	}

	duration, err := time.ParseDuration(cmd.args[0])
	if err != nil {
		return err
	}

	fmt.Printf("Collecting feeds every %s\n", duration)

	ticker := time.NewTicker(duration)
	for ; ; <-ticker.C {
		err = scrapeFeeds(s)
		if err != nil {
			return err
		}
	}
}

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	f := func(s *state, cmd command) error {
		currentUser, err := s.db.GetUser(context.Background(), s.cfg.CurrentUserName)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		handler(s, cmd, currentUser)
		return nil
	}

	return f
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
	commandsStc.register("users", handlerListUsers)
	commandsStc.register("reset", handlerReset)
	commandsStc.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	commandsStc.register("feeds", handlerListFeeds)
	commandsStc.register("follow", middlewareLoggedIn(handlerFollow))
	commandsStc.register("unfollow", middlewareLoggedIn(handlerUnfollow))
	commandsStc.register("following", middlewareLoggedIn(handlerListFollows))
	commandsStc.register("agg", handlerAgg)

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
