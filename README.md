# go_taskmaster
Alec Bueing
ajb497
June 5th, 2023


This is my first time coding in go, so there are likely some inefficiencies in my design.
The basic architecture follows the example of your "goquic" repo, and the basic connection management components are largely copies of what you had in that example.
The "ptmp" folder contains the serialization/deserialization functions for the messages.

Per your recommendation in response to my protocol definition, I've slimmed down the implementation by omitting some messages from this demonstration, so messages defined in my protocol pertaining to List management have not been implemented.

