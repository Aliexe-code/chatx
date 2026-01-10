        document.addEventListener('DOMContentLoaded', function() {
            let ws = null;
            let currentRoom = "default";
            let availableRooms = { default: [], private: [] };
            const messagesDiv = document.getElementById('messages');
            let currentUsername = "";
            const messageInput = document.getElementById('messageInput');
            const sendButton = document.getElementById('sendButton');
            const connectButton = document.getElementById('connectButton');
            const statusDiv = document.getElementById('status');
            const currentRoomSpan = document.getElementById('currentRoom');
            const createRoomButton = document.getElementById('createRoom');
            const leaveRoomButton = document.getElementById('leaveRoom');
            const listRoomsButton = document.getElementById('listRooms');
            const defaultRoomsDiv = document.getElementById('defaultRooms');
            const privateRoomsDiv = document.getElementById('privateRooms');

            // Modal elements
            const createRoomModal = document.getElementById('createRoomModal');
            const closeModal = document.getElementById('closeModal');
            const cancelCreateRoom = document.getElementById('cancelCreateRoom');
            const confirmCreateRoom = document.getElementById('confirmCreateRoom');
            const newRoomNameInput = document.getElementById('newRoomName');
            const privateRoomCheckbox = document.getElementById('privateRoom');
            const passwordField = document.getElementById('passwordField');
            const roomPasswordInput = document.getElementById('roomPassword');

            const joinRoomModal = document.getElementById('joinRoomModal');
            const closeJoinModal = document.getElementById('closeJoinModal');
            const cancelJoinRoom = document.getElementById('cancelJoinRoom');
            const confirmJoinRoomButton = document.getElementById('confirmJoinRoom');
            const joinRoomNameInput = document.getElementById('joinRoomName');
            const joinRoomNameDisplay = document.getElementById('joinRoomNameDisplay');
            const joinPasswordField = document.getElementById('joinPasswordField');
            const joinRoomPasswordInput = document.getElementById('joinRoomPassword');

            const deleteRoomModal = document.getElementById('deleteRoomModal');
            const closeDeleteModal = document.getElementById('closeDeleteModal');
            const cancelDeleteRoom = document.getElementById('cancelDeleteRoom');
            const confirmDeleteRoom = document.getElementById('confirmDeleteRoom');
            const deleteRoomNameDisplay = document.getElementById('deleteRoomNameDisplay');
            let roomToDelete = '';

            function addMessage(content, type = 'user', isSent = false) {
                const messageDiv = document.createElement('div');
                
                // Check if message is from current user
                if (type === 'user' && currentUsername && content.includes(currentUsername + ':')) {
                    isSent = true;
                }
                
                // Apply appropriate class
                if (isSent) {
                    messageDiv.className = 'message sent';
                } else if (type === 'user') {
                    messageDiv.className = 'message received';
                } else {
                    messageDiv.className = `message ${type}`;
                }

                if (content.includes('has joined the room')) {
                    messageDiv.className = 'message room_join';
                } else if (content.includes('has left the room')) {
                    messageDiv.className = 'message room_leave';
                } else if (content.includes('Welcome to the chat')) {
                    messageDiv.className = 'message system';
                    // Extract username from welcome message
                    const match = content.match(/Your name is (User\d+)/);
                    if (match) {
                        currentUsername = match[1];
                    }
                } else if (content.includes('ROOMS_LIST:')) {
                    messageDiv.className = 'message system';
                } else if (content.includes('Error')) {
                    messageDiv.className = 'message system';
                }

                messageDiv.textContent = content;
                messagesDiv.appendChild(messageDiv);
                messagesDiv.scrollTop = messagesDiv.scrollHeight;
            }

            function updateStatus(connected) {
                if (connected) {
                    statusDiv.textContent = 'Connected';
                    statusDiv.className = 'status connected';
                    messageInput.disabled = false;
                    sendButton.disabled = false;
                    connectButton.textContent = 'Disconnect';
                } else {
                    statusDiv.textContent = 'Disconnected';
                    statusDiv.className = 'status disconnected';
                    messageInput.disabled = true;
                    sendButton.disabled = true;
                    connectButton.textContent = 'Connect';
                }
            }

            function updateCurrentRoom(roomName) {
                currentRoom = roomName;
                currentRoomSpan.textContent = `Current Room: ${roomName}`;
            }

            function updateRoomList(roomsData) {
                availableRooms = { default: [], private: [] };
                
                roomsData.forEach(room => {
                    if (room.private) {
                        availableRooms.private.push(room);
                    } else {
                        availableRooms.default.push(room);
                    }
                });

                renderRoomList();
            }

            function renderRoomList() {
                defaultRoomsDiv.innerHTML = '';
                privateRoomsDiv.innerHTML = '';

                availableRooms.default.forEach(room => {
                    const roomDiv = document.createElement('div');
                    roomDiv.className = `room-item ${room.name === currentRoom ? 'active' : ''}`;
                    let deleteButton = '';
                    if (room.isCreator && room.name !== 'default') {
                        deleteButton = `<button class="delete-room-btn" data-room="${room.name}">Delete</button>`;
                    }
                    roomDiv.innerHTML = `
                        <div class="room-item-name">
                            <span>${room.name}</span>
                            <span>${room.clientCount} users ${deleteButton}</span>
                        </div>
                        <div class="room-item-info">Public room</div>
                    `;
                    roomDiv.addEventListener('click', function(e) {
                        if (e.target.classList.contains('delete-room-btn')) {
                            e.stopPropagation();
                            showDeleteRoomModal(room.name);
                        } else {
                            joinRoomByName(room.name, false);
                        }
                    });
                    defaultRoomsDiv.appendChild(roomDiv);
                });

                availableRooms.private.forEach(room => {
                    const roomDiv = document.createElement('div');
                    roomDiv.className = `room-item ${room.name === currentRoom ? 'active' : ''}`;
                    let deleteButton = '';
                    if (room.isCreator) {
                        deleteButton = `<button class="delete-room-btn" data-room="${room.name}">Delete</button>`;
                    }
                    roomDiv.innerHTML = `
                        <div class="room-item-name">
                            <span>${room.name} <span class="private-badge">Private</span></span>
                            <span>${room.clientCount} users ${deleteButton}</span>
                        </div>
                        <div class="room-item-info">Password protected</div>
                    `;
                    roomDiv.addEventListener('click', function(e) {
                        if (e.target.classList.contains('delete-room-btn')) {
                            e.stopPropagation();
                            showDeleteRoomModal(room.name);
                        } else {
                            showJoinRoomModal(room.name, true);
                        }
                    });
                    privateRoomsDiv.appendChild(roomDiv);
                });
            }

            function connect() {
                if (ws) {
                    ws.close();
                    ws = null;
                    updateStatus(false);
                    return;
                }

                ws = new WebSocket('ws://localhost:8080/ws');

                ws.onopen = function(event) {
                    updateStatus(true);
                    messagesDiv.innerHTML = '';
                    addMessage('Connected to WebSocket server', 'system');
                    updateCurrentRoom('default');
                    listRooms();
                };

                ws.onmessage = function(event) {
                    if (event.data.includes('ROOMS_LIST:')) {
                        const jsonStr = event.data.replace('ROOMS_LIST:', '');
                        try {
                            const roomsData = JSON.parse(jsonStr);
                            updateRoomList(roomsData);
                        } catch (e) {
                            // Silently handle parsing errors
                        }
                    } else {
                        // Check if message has room prefix and if it matches current room
                        const roomPrefixMatch = event.data.match(/^\[([^\]]+)\]/);
                        if (roomPrefixMatch) {
                            const messageRoom = roomPrefixMatch[1];
                            // Only show messages from current room
                            if (messageRoom === currentRoom) {
                                addMessage(event.data);
                            }
                        } else {
                            // Messages without room prefix - show them
                            addMessage(event.data);
                        }
                    }
                };

                ws.onclose = function(event) {
                    updateStatus(false);
                    addMessage('Disconnected from WebSocket server', 'system');
                    ws = null;
                };

                ws.onerror = function(error) {
                    addMessage(`WebSocket error occurred`, 'system');
                };
            }

            function sendMessage() {
                const message = messageInput.value.trim();
                if (message && ws && ws.readyState === WebSocket.OPEN) {
                    const wsMsg = {
                        type: 'room_message',
                        data: { content: message }
                    };
                    
                    ws.send(JSON.stringify(wsMsg));
                    messageInput.value = '';
                }
            }

            function showCreateRoomModal() {
                createRoomModal.classList.add('show');
                newRoomNameInput.focus();
            }

            function hideCreateRoomModal() {
                createRoomModal.classList.remove('show');
                newRoomNameInput.value = '';
                privateRoomCheckbox.checked = false;
                roomPasswordInput.value = '';
                passwordField.classList.remove('show');
            }

            function createRoom() {
                const roomName = newRoomNameInput.value.trim();
                const isPrivate = privateRoomCheckbox.checked;
                const password = roomPasswordInput.value.trim();

                if (!roomName) {
                    alert('Please enter a room name');
                    return;
                }

                if (isPrivate && !password) {
                    alert('Please enter a password for private room');
                    return;
                }

                if (ws && ws.readyState === WebSocket.OPEN) {
                    const wsMsg = {
                        type: 'create_room',
                        data: { 
                            name: roomName,
                            private: isPrivate,
                            password: password
                        }
                    };
                    ws.send(JSON.stringify(wsMsg));
                    hideCreateRoomModal();
                    listRooms();
                }
            }

            function showJoinRoomModal(roomName, isPrivate) {
                joinRoomNameInput.value = roomName;
                joinRoomNameDisplay.textContent = roomName;
                
                if (isPrivate) {
                    joinPasswordField.classList.add('show');
                    joinRoomPasswordInput.value = '';
                    joinRoomPasswordInput.focus();
                } else {
                    joinPasswordField.classList.remove('show');
                    joinRoomByName(roomName, false);
                    return;
                }
                
                joinRoomModal.classList.add('show');
            }

            function hideJoinRoomModal() {
                joinRoomModal.classList.remove('show');
                joinRoomPasswordInput.value = '';
            }

            function confirmJoinRoom() {
                const roomName = joinRoomNameInput.value;
                const password = joinRoomPasswordInput.value.trim();
                
                if (password) {
                    joinRoomByName(roomName, true, password);
                } else {
                    alert('Please enter the password');
                }
                hideJoinRoomModal();
            }

            function joinRoomByName(roomName, isPrivate, password = '') {
                if (ws && ws.readyState === WebSocket.OPEN) {
                    const wsMsg = {
                        type: 'join_room',
                        data: { 
                            name: roomName,
                            password: password
                        }
                    };
                    ws.send(JSON.stringify(wsMsg));
                    updateCurrentRoom(roomName);
                    listRooms();
                    // Clear messages when switching rooms
                    messagesDiv.innerHTML = '';
                    addMessage(`Joined room: ${roomName}`, 'system');
                }
            }

            function showDeleteRoomModal(roomName) {
                roomToDelete = roomName;
                deleteRoomNameDisplay.textContent = roomName;
                deleteRoomModal.classList.add('show');
            }

            function hideDeleteRoomModal() {
                deleteRoomModal.classList.remove('show');
                roomToDelete = '';
            }

            function deleteRoom() {
                if (ws && ws.readyState === WebSocket.OPEN && roomToDelete) {
                    const wsMsg = {
                        type: 'delete_room',
                        data: { 
                            name: roomToDelete
                        }
                    };
                    ws.send(JSON.stringify(wsMsg));
                    hideDeleteRoomModal();
                    listRooms();
                }
            }

            function leaveRoom() {
                if (ws && ws.readyState === WebSocket.OPEN) {
                    // If already in default room, don't leave
                    if (currentRoom === 'default') {
                        addMessage('You are already in the default room', 'system');
                        return;
                    }
                    
                    const wsMsg = {
                        type: 'leave_room',
                        data: {}
                    };
                    ws.send(JSON.stringify(wsMsg));
                    
                    // Clear messages and join default room after leaving
                    messagesDiv.innerHTML = '';
                    setTimeout(function() {
                        joinRoomByName('default', false);
                    }, 100);
                }
            }

            function listRooms() {
                if (ws && ws.readyState === WebSocket.OPEN) {
                    const wsMsg = {
                        type: 'list_rooms',
                        data: {}
                    };
                    ws.send(JSON.stringify(wsMsg));
                }
            }

            // Event listeners
            connectButton.addEventListener('click', connect);
            sendButton.addEventListener('click', sendMessage);
            messageInput.addEventListener('keypress', function(event) {
                if (event.key === 'Enter') {
                    sendMessage();
                }
            });

            createRoomButton.addEventListener('click', showCreateRoomModal);
            leaveRoomButton.addEventListener('click', leaveRoom);
            listRoomsButton.addEventListener('click', listRooms);

            // Modal event listeners
            closeModal.addEventListener('click', hideCreateRoomModal);
            cancelCreateRoom.addEventListener('click', hideCreateRoomModal);
            confirmCreateRoom.addEventListener('click', createRoom);

            closeJoinModal.addEventListener('click', hideJoinRoomModal);
            cancelJoinRoom.addEventListener('click', hideJoinRoomModal);
            confirmJoinRoomButton.addEventListener('click', confirmJoinRoom);

            closeDeleteModal.addEventListener('click', hideDeleteRoomModal);
            cancelDeleteRoom.addEventListener('click', hideDeleteRoomModal);
            confirmDeleteRoom.addEventListener('click', deleteRoom);

            // Close modal on outside click
            createRoomModal.addEventListener('click', function(event) {
                if (event.target === createRoomModal) {
                    hideCreateRoomModal();
                }
            });

            joinRoomModal.addEventListener('click', function(event) {
                if (event.target === joinRoomModal) {
                    hideJoinRoomModal();
                }
            });

            deleteRoomModal.addEventListener('click', function(event) {
                if (event.target === deleteRoomModal) {
                    hideDeleteRoomModal();
                }
            });

            // Private room checkbox toggle
            privateRoomCheckbox.addEventListener('change', function() {
                if (this.checked) {
                    passwordField.classList.add('show');
                } else {
                    passwordField.classList.remove('show');
                    roomPasswordInput.value = '';
                }
            });

            // Enter key in password fields
            roomPasswordInput.addEventListener('keypress', function(event) {
                if (event.key === 'Enter') {
                    createRoom();
                }
            });

            joinRoomPasswordInput.addEventListener('keypress', function(event) {
                if (event.key === 'Enter') {
                    confirmJoinRoom();
                }
            });

            updateStatus(false);
            addMessage('WebSocket demo loaded. Click "Connect" to start.', 'system');
        });
