package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/urfave/cli/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/gookit/color.v1"
	"log"
	"os"
	"strconv"
	"time"
)

var collection *mongo.Collection
var ctx = context.TODO()

func createTask(task *Task) error {
	_, err := collection.InsertOne(ctx, task)
	return err
}

func init() {
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017/")
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	collection = client.Database("tasker").Collection("tasks")
}

func getAll() ([]*Task, error) {
	filter := bson.D{{}}
	return filterTasks(filter)
}

func filterTasks(filter interface{}) ([]*Task, error) {
	var tasks []*Task

	findOptions := options.Find()
	findOptions.SetSort(bson.D{{Key: "priority", Value: -1}})

	cur, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		return tasks, err
	}

	for cur.Next(ctx) {
		var t Task
		err := cur.Decode(&t)
		if err != nil {
			return tasks, err
		}

		tasks = append(tasks, &t)
	}

	if err := cur.Err(); err != nil {
		return tasks, err
	}

	_ = cur.Close(ctx)

	if len(tasks) == 0 {
		return tasks, mongo.ErrNoDocuments
	}

	return tasks, nil
}

func printTasks(tasks []*Task) {
	for i, v := range tasks {
		if v.Completed {
			color.Green.Printf("%d: %s\n", i+1, v.Text)
		} else {
			color.Yellow.Printf("%d: %s\n", i+1, v.Text)
		}
	}
}

func printTasksWithPriority(tasks []*Task) {
	for i, v := range tasks {
		if v.Completed {
			color.Green.Printf("%d: %s | Приоритет: %d\n", i+1, v.Text, v.Priority)
		} else {
			color.Yellow.Printf("%d: %s | Приоритет: %d\n", i+1, v.Text, v.Priority)
		}
	}
}

func completeTask(text string) error {
	filter := bson.D{primitive.E{Key: "text", Value: text}}

	update := bson.D{primitive.E{Key: "$set", Value: bson.D{
		primitive.E{Key: "completed", Value: true},
	}}}

	t := &Task{}
	return collection.FindOneAndUpdate(ctx, filter, update).Decode(t)
}

func getPending() ([]*Task, error) {
	filter := bson.D{
		primitive.E{Key: "completed", Value: false},
	}

	return filterTasks(filter)
}

func getFinished() ([]*Task, error) {
	filter := bson.D{
		primitive.E{Key: "completed", Value: true},
	}

	return filterTasks(filter)
}

func deleteTask(text string) error {
	filter := bson.D{primitive.E{Key: "text", Value: text}}

	res, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}

	if res.DeletedCount == 0 {
		return errors.New("Ни одна задача не удалена")
	}

	return nil
}

func setPriority(text string, priority int) error {
	filter := bson.D{primitive.E{Key: "text", Value: text}}

	update := bson.D{primitive.E{Key: "$set", Value: bson.D{
		primitive.E{Key: "priority", Value: priority},
	}}}

	t := &Task{}
	return collection.FindOneAndUpdate(ctx, filter, update).Decode(t)
}

func main() {
	app := &cli.App{
		Name:  "polivoda",
		Usage: "Простая консольная утилита, управляющая задачами. Работает с MongoDB.",
		Action: func(c *cli.Context) error {
			tasks, err := getPending()
			if err != nil {
				if err == mongo.ErrNoDocuments {
					fmt.Println("Тут ничего нет.\nЗапустите `add 'task'`, чтобы добавить задачу.")
					return nil
				}

				return err
			}

			printTasks(tasks)
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:    "add",
				Aliases: []string{"a"},
				Usage:   "создать задачу в базе данных",
				Action: func(c *cli.Context) error {
					str := c.Args().First()
					if str == "" {
						return errors.New("Нельзя создать пустую задачу")
					}

					task := &Task{
						ID:        primitive.NewObjectID(),
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
						Text:      str,
						Completed: false,
						Priority:  1,
					}

					return createTask(task)
				},
			},
			{
				Name:    "all",
				Aliases: []string{"l"},
				Usage:   "получить список всех задач",
				Action: func(c *cli.Context) error {
					tasks, err := getAll()
					if err != nil {
						if err == mongo.ErrNoDocuments {
							fmt.Println("Тут ничего нет.\nЗапустите `add 'task'`, чтобы создать задачу.")
							return nil
						}

						return err
					}

					printTasksWithPriority(tasks)
					return nil
				},
			},
			{
				Name:    "done",
				Aliases: []string{"d"},
				Usage:   "пометить задачу как выполненную",
				Action: func(c *cli.Context) error {
					text := c.Args().First()
					return completeTask(text)
				},
			},
			{
				Name:    "finished",
				Aliases: []string{"f"},
				Usage:   "получить список всех выполненных задач",
				Action: func(c *cli.Context) error {
					tasks, err := getFinished()
					if err != nil {
						if err == mongo.ErrNoDocuments {
							fmt.Println("Тут ничего нет.\nЗапустите `done 'task'`, чтобы пометить задачу как выполненную.")
							return nil
						}

						return err
					}

					printTasks(tasks)
					return nil
				},
			},
			{
				Name:    "unfinished",
				Aliases: []string{"u"},
				Usage:   "получить список всех невыполненных задач",
				Action: func(c *cli.Context) error {
					tasks, err := getPending()
					if err != nil {
						if err == mongo.ErrNoDocuments {
							fmt.Println("Тут ничего нет.")
							return nil
						}

						return err
					}

					printTasks(tasks)
					return nil
				},
			},
			{
				Name:    "remove",
				Aliases: []string{"r"},
				Usage:   "удалить задачу в списке",
				Action: func(c *cli.Context) error {
					text := c.Args().First()
					err := deleteTask(text)
					if err != nil {
						return err
					}

					return nil
				},
			},
			{
				Name:        "priority",
				Aliases:     []string{"p"},
				Usage:       "задать приоритет задаче в списке",
				Description: "Приоритет должен быть неотрицательным числом",
				Action: func(c *cli.Context) error {
					text := c.Args().First()
					priority, errInt := strconv.Atoi(c.Args().Get(1))
					if errInt != nil || priority < 0 {
						return errors.New("Задан некорректный приоритет")
					}

					err := setPriority(text, priority)
					if err != nil {
						return err
					}

					return nil
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
