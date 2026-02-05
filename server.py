from flask import Flask, send_from_directory, jsonify, request
from flask_cors import CORS
import os

app = Flask(__name__, static_folder='.')
CORS(app)

# Mock API data
tracks = [
    {"id": 1, "title": "Industrial Silence", "artist": "Iron Beat", "duration": 245},
    {"id": 2, "title": "Neon Dreams", "artist": "Cyber Girl", "duration": 180},
    {"id": 3, "title": "Concrete Jungle", "artist": "Urban Explorer", "duration": 312}
]

users = [
    {"id": 1, "username": "admin", "displayName": "Administrator", "role": "admin"},
    {"id": 2, "username": "user", "displayName": "Regular User", "role": "user"}
]

@app.route('/')
def index():
    return send_from_directory('.', 'index.html')

@app.route('/<path:path>')
def static_files(path):
    return send_from_directory('.', path)

@app.route('/api/tracks')
def get_tracks():
    return jsonify(tracks)

@app.route('/api/users')
def get_users():
    return jsonify(users)

@app.route('/api/auth/login', methods=['POST'])
def login():
    data = request.json
    username = data.get('username')
    password = data.get('password')
    
    # Very simple mock auth
    user = next((u for u in users if u['username'] == username), None)
    if user and password == username: # In this mock, password is same as username
        return jsonify({
            "token": f"mock-jwt-token-{username}",
            "user": user
        })
    return jsonify({"error": "Invalid credentials"}), 401

# SPA handling: redirect all unknown routes to index.html
@app.errorhandler(404)
def not_found(e):
    return send_from_directory('.', 'index.html')

if __name__ == '__main__':
    app.run(port=5000, debug=True)
