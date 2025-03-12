// Create application database and user
db = db.getSiblingDB(process.env.MONGO_INITDB_DATABASE || 'nxpose');

// Create application user with appropriate permissions
db.createUser({
  user: process.env.MONGO_APP_USERNAME || 'nxpose',
  pwd: process.env.MONGO_APP_PASSWORD || 'nxpose-password',
  roles: [
    { role: 'readWrite', db: process.env.MONGO_INITDB_DATABASE || 'nxpose' }
  ]
});

// Create initial collections
db.createCollection('users');
db.createCollection('clients');
db.createCollection('tunnels');
db.createCollection('sessions');

// Create indexes for better performance
db.users.createIndex({ "email": 1 }, { unique: true });
db.clients.createIndex({ "client_id": 1 }, { unique: true });
db.tunnels.createIndex({ "tunnel_id": 1 }, { unique: true });
db.tunnels.createIndex({ "user_id": 1 });
db.tunnels.createIndex({ "client_id": 1 });
db.sessions.createIndex({ "expires_at": 1 }, { expireAfterSeconds: 0 });

print('MongoDB initialization completed'); 