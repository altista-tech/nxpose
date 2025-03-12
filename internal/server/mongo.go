package server

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoConfig represents the configuration for MongoDB
type MongoConfig struct {
	URI      string
	Database string
	Timeout  time.Duration
}

// User represents a user in the database
type User struct {
	ID           string    `bson:"_id"`
	Email        string    `bson:"email"`
	Name         string    `bson:"name"`
	Picture      string    `bson:"picture,omitempty"`
	RegisteredAt time.Time `bson:"registered_at"`
	LastLoginAt  time.Time `bson:"last_login_at"`
	Provider     string    `bson:"provider"`
	ProviderID   string    `bson:"provider_id"`
}

// Certificate represents a client certificate in the database
type Certificate struct {
	ID          string    `bson:"_id"`
	UserID      string    `bson:"user_id"`
	Certificate []byte    `bson:"certificate"`
	PrivateKey  []byte    `bson:"private_key"`
	IssuedAt    time.Time `bson:"issued_at"`
	ExpiresAt   time.Time `bson:"expires_at"`
}

// MongoClient represents a MongoDB client
type MongoClient struct {
	client   *mongo.Client
	database *mongo.Database
	log      *logrus.Logger
	users    *mongo.Collection
	tunnels  *mongo.Collection
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewMongoClient creates a new MongoDB client
func NewMongoClient(config MongoConfig) (*MongoClient, error) {
	// Set default timeout if not specified
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Connect to MongoDB
	clientOptions := options.Client().ApplyURI(config.URI)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Check the connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	// Get database
	database := client.Database(config.Database)

	return &MongoClient{
		client:   client,
		database: database,
		log:      logrus.StandardLogger(),
		users:    database.Collection("users"),
		tunnels:  database.Collection("tunnels"),
		ctx:      ctx,
		cancel:   cancel,
	}, nil
}

// Close closes the MongoDB connection
func (m *MongoClient) Close() error {
	m.cancel()
	return m.client.Disconnect(context.Background())
}

// FindUserByID finds a user by ID
func (m *MongoClient) FindUserByID(ctx context.Context, id string) (*User, error) {
	collection := m.database.Collection("users")
	var user User
	err := collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// FindUserByEmail finds a user by email address
func (m *MongoClient) FindUserByEmail(ctx context.Context, email string) (*User, error) {
	collection := m.database.Collection("users")
	var user User
	err := collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// FindUserByProviderID finds a user by provider and provider ID
func (m *MongoClient) FindUserByProviderID(ctx context.Context, provider, providerID string) (*User, error) {
	collection := m.database.Collection("users")
	var user User
	err := collection.FindOne(ctx, bson.M{
		"provider":    provider,
		"provider_id": providerID,
	}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// CreateUser creates a new user
func (m *MongoClient) CreateUser(ctx context.Context, user *User) error {
	collection := m.database.Collection("users")
	_, err := collection.InsertOne(ctx, user)
	return err
}

// UpdateUser updates an existing user
func (m *MongoClient) UpdateUser(ctx context.Context, user *User) error {
	collection := m.database.Collection("users")
	_, err := collection.ReplaceOne(ctx, bson.M{"_id": user.ID}, user)
	return err
}

// SaveCertificate saves a certificate to the database
func (m *MongoClient) SaveCertificate(ctx context.Context, cert *Certificate) error {
	collection := m.database.Collection("certificates")
	_, err := collection.InsertOne(ctx, cert)
	return err
}

// FindCertificateByUserID finds a certificate by user ID
func (m *MongoClient) FindCertificateByUserID(ctx context.Context, userID string) (*Certificate, error) {
	collection := m.database.Collection("certificates")
	var cert Certificate
	err := collection.FindOne(ctx, bson.M{"user_id": userID}).Decode(&cert)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &cert, nil
}

// ListCertificates lists all certificates
func (m *MongoClient) ListCertificates(ctx context.Context) ([]*Certificate, error) {
	collection := m.database.Collection("certificates")
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var certs []*Certificate
	if err := cursor.All(ctx, &certs); err != nil {
		return nil, err
	}
	return certs, nil
}
