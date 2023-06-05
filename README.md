# go_taskmaster
Alec Bueing
ajb497
June 5th, 2023


This is my first time coding in go, so there are certainly some inefficiencies in my design.

Per your recommendation in response to my protocol definition, I've slimmed down the implementation by limting the final version to the connection management messages and the task management messages, with those pertaining to list management omitted.  Additionally, the component of the connection setup messages pertaining to timeout has not been implemented.

On a linux system, a demonstration of the protocol can be executed by sourcing the "run_proj.sh" script located in the root directory of the project.  Ensure that the script is being called from the root directory of the project.
The 'run_proj.sh' script launches the server as a background process and then launches the client.
The client relies on a configuration file (client.cfg, located in the client directory) to determine the host and port number to connect to.
The configuration file has a second line in it by default with the word "DEMO" on that line.  If you delete that line, then running the client will prompt you for user inputs.  The only accepted username and password combo at the moment is "Ed Ucational" and "p@55w0rd", so if you run th e client in non-demo mode, that's what you need to use in order to get the connection established per the protocol.
In demo mode, the client runs through an example session with the server, including some messages intended to generate error responses from the server.
The server, once started, awaits a connection from the client and will respond to messages per the DFA from the protocol design document.


The 'ptmp' folder contains the common library utilized by both the client and the server to define the messages used in the protocol and to handle encoding/decoding to/from byte arrays.
The server is configured to listen for a TCP connection on 'localhost:10101'.


The basic architecture follows the example of your "goquic" repo, with the quic protocol omitted and replaced with simple TCP so as to avoid utilizing third party libraries for the connection.

Feedback to the protocol design from the implementation:
The original design of the protocol called for concatenating the payloads of a series of messages in order to generate the full content of a message transaction, but during the implementation stage, I determined this was over-complicating the system.  Simply limiting each message to carry a single data structure in the payload (thinking specifically of the Task Information messages) greatly simplified the implementation and allowed for a more straightforward reception process instead of setting up a system to buffer inputs and await the full transmission to begin decoding (which would also lead to some memory size uncertainty).
An additional feedback item to myself as the designer of the protocol is that the element of the header specifying the payload size is not really necessary since the payload size is
deterministic based on the message type (with some help from fields within the payloads themselves), so having that field does add an extra step to creating the message that doesn't actually need to be there.
