

def init(self,arg="test"):
    self.__class__.name = "hello"
    self.__class__.maintainers = [ {"name" : "kramer" } ]
    self.__class__.version = "1.0.0"
    self.__class__.app_version = "1.0.0"
    self.__class__.description = "Hello world"
    self.message = property(default = "Hello World")
    self.arg = arg

